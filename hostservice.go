package main

import (
	"fmt"
	"strings"
	"sync"

	"go-ssh/config"
	"go-ssh-ui/internal/configx"
)

// HostService is the Wails-bound entry point for category/host CRUD against
// the shared ~/.go-ssh/config.yaml + conf.d tree. It mirrors go-ssh's
// web/handlers_config.go handlers (same request shapes, same
// validate-then-save-then-reload flow) but returns Go values directly
// instead of JSON over HTTP - there is no separate auth/CSRF layer here
// because, unlike go-ssh's web UI, this service is never reachable except
// from this app's own window.
type HostService struct {
	mu  sync.Mutex
	cfg *config.Config
}

func (s *HostService) reloadLocked() error {
	cfg, err := configx.LoadConfig()
	if err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}

// GetTree returns the current category/host tree, loading it on first call.
func (s *HostService) GetTree() (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		if err := s.reloadLocked(); err != nil {
			return nil, err
		}
	}
	return s.cfg, nil
}

// Reload re-reads config.yaml + conf.d/*.yaml from disk, discarding any
// in-memory state - used after external changes (e.g. go-ssh --edit running
// concurrently).
func (s *HostService) Reload() (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.reloadLocked(); err != nil {
		return nil, err
	}
	return s.cfg, nil
}

// GetFiles reports the config.yaml/conf.d layout for the target-file picker.
func (s *HostService) GetFiles() (configx.Files, error) {
	return configx.GetFiles()
}

// FindHostByID looks up a host by its stable ID - used to resolve jump-host
// references (Host.JumpVia). Implements sshengine.HostFinder.
func (s *HostService) FindHostByID(id string) (config.Host, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		if err := s.reloadLocked(); err != nil {
			return config.Host{}, false
		}
	}
	return configx.FindHostByID(s.cfg, id)
}

// FindHostByPath looks up a host by category path + name, the addressing
// the tree UI already uses - unlike FindHostByID this works even for hosts
// that were never saved through go-ssh-ui and so have no ID yet.
func (s *HostService) FindHostByPath(categoryPath []string, name string) (config.Host, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		if err := s.reloadLocked(); err != nil {
			return config.Host{}, false
		}
	}
	return configx.FindHostByPath(s.cfg, categoryPath, name)
}

// AddCategoryRequest mirrors go-ssh's web addCategoryRequest.
type AddCategoryRequest struct {
	TargetFile  string   `json:"targetFile"`
	ParentPath  []string `json:"parentPath"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
}

func (s *HostService) AddCategory(req AddCategoryRequest) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	targetFile, err := configx.ValidateFilePath(req.TargetFile)
	if err != nil {
		return nil, err
	}
	category, err := configx.NormalizeCategory(config.Category{Name: req.Name, Description: req.Description})
	if err != nil {
		return nil, err
	}
	if err := config.SaveCategoryToFile(targetFile, req.ParentPath, category); err != nil {
		return nil, err
	}
	if err := s.reloadLocked(); err != nil {
		return nil, err
	}
	return s.cfg, nil
}

// EditCategoryRequest mirrors go-ssh's web editCategoryRequest.
type EditCategoryRequest struct {
	SourceFile   string   `json:"sourceFile"`
	CategoryPath []string `json:"categoryPath"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
}

func (s *HostService) EditCategory(req EditCategoryRequest) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.CategoryPath) == 0 {
		return nil, fmt.Errorf("category path is required")
	}
	sourceFile, err := configx.ValidateFilePath(req.SourceFile)
	if err != nil {
		return nil, err
	}
	category, err := configx.NormalizeCategory(config.Category{Name: req.Name, Description: req.Description})
	if err != nil {
		return nil, err
	}

	// Rename first (preserves children in place), then save the description
	// onto the now-correctly-named category.
	oldName := req.CategoryPath[len(req.CategoryPath)-1]
	if oldName != category.Name {
		if err := config.RenameCategoryInFile(sourceFile, req.CategoryPath, category.Name); err != nil {
			return nil, err
		}
	}
	parentPath := req.CategoryPath[:len(req.CategoryPath)-1]
	if err := config.SaveCategoryToFile(sourceFile, parentPath, category); err != nil {
		return nil, err
	}
	if err := s.reloadLocked(); err != nil {
		return nil, err
	}
	return s.cfg, nil
}

// DeleteCategoryRequest mirrors go-ssh's web deleteCategoryRequest.
type DeleteCategoryRequest struct {
	SourceFile   string   `json:"sourceFile"`
	CategoryPath []string `json:"categoryPath"`
}

func (s *HostService) DeleteCategory(req DeleteCategoryRequest) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.CategoryPath) == 0 {
		return nil, fmt.Errorf("category path is required")
	}
	// Deletes from every file that defines this category, not just
	// req.SourceFile - MergeCategoriesByName can fold the same root category
	// in from several conf.d files into one tree node, and a single-file
	// removal left the other files' copies reappearing on reload.
	if err := config.RemoveCategoryEverywhere(req.CategoryPath); err != nil {
		return nil, err
	}
	if err := s.reloadLocked(); err != nil {
		return nil, err
	}
	return s.cfg, nil
}

// AddHostRequest mirrors go-ssh's web addHostRequest.
type AddHostRequest struct {
	TargetFile   string      `json:"targetFile"`
	CategoryPath []string    `json:"categoryPath"`
	Host         config.Host `json:"host"`
}

func (s *HostService) AddHost(req AddHostRequest) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.CategoryPath) == 0 {
		return nil, fmt.Errorf("host must be added under a category")
	}
	targetFile, err := configx.ValidateFilePath(req.TargetFile)
	if err != nil {
		return nil, err
	}
	host, err := configx.NormalizeHost(req.Host)
	if err != nil {
		return nil, err
	}
	if err := config.SaveHostToFile(targetFile, req.CategoryPath, host); err != nil {
		return nil, err
	}
	if err := s.reloadLocked(); err != nil {
		return nil, err
	}
	return s.cfg, nil
}

// EditHostRequest mirrors go-ssh's web editHostRequest.
type EditHostRequest struct {
	SourceFile   string      `json:"sourceFile"`
	CategoryPath []string    `json:"categoryPath"`
	OriginalName string      `json:"originalName"`
	Host         config.Host `json:"host"`
}

func (s *HostService) EditHost(req EditHostRequest) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.CategoryPath) == 0 {
		return nil, fmt.Errorf("host must belong to a category")
	}
	sourceFile, err := configx.ValidateFilePath(req.SourceFile)
	if err != nil {
		return nil, err
	}
	host, err := configx.NormalizeHost(req.Host)
	if err != nil {
		return nil, err
	}

	if req.OriginalName != "" && req.OriginalName != host.Name {
		if err := config.RemoveHostFromFile(sourceFile, req.CategoryPath, req.OriginalName); err != nil {
			return nil, err
		}
	}
	if err := config.SaveHostToFile(sourceFile, req.CategoryPath, host); err != nil {
		return nil, err
	}
	if err := s.reloadLocked(); err != nil {
		return nil, err
	}
	return s.cfg, nil
}

// DeleteHostRequest mirrors go-ssh's web deleteHostRequest.
type DeleteHostRequest struct {
	SourceFile   string   `json:"sourceFile"`
	CategoryPath []string `json:"categoryPath"`
	Name         string   `json:"name"`
}

func (s *HostService) DeleteHost(req DeleteHostRequest) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.CategoryPath) == 0 || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("category path and host name are required")
	}
	sourceFile, err := configx.ValidateFilePath(req.SourceFile)
	if err != nil {
		return nil, err
	}
	if err := config.RemoveHostFromFile(sourceFile, req.CategoryPath, req.Name); err != nil {
		return nil, err
	}
	if err := s.reloadLocked(); err != nil {
		return nil, err
	}
	return s.cfg, nil
}
