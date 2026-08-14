package configx

import (
	"fmt"
	"path/filepath"
	"strings"

	"go-ssh/config"
)

// ValidateFilePath confines a targetFile/sourceFile value coming from the
// frontend to the real config.yaml, an existing conf.d/*.yaml|*.yml file, or
// a not-yet-existing direct child of conf.d/ (for creating a new modular
// config file). Ported from go-ssh's web/validate.go: config.SaveCategoryToFile
// and friends trust their path argument completely, so this check is what
// keeps a buggy or compromised frontend call from pointing a save/delete at
// an arbitrary file on disk. Returns the cleaned path to use for the actual
// file operation.
func ValidateFilePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("file path is required")
	}
	clean := filepath.Clean(path)

	configPath, err := config.GetConfigPath()
	if err != nil {
		return "", err
	}
	if clean == configPath {
		return clean, nil
	}

	confDFiles, err := config.ListConfDFiles()
	if err != nil {
		return "", err
	}
	for _, f := range confDFiles {
		if clean == f {
			return clean, nil
		}
	}

	confDDir, err := config.GetConfDDir()
	if err != nil {
		return "", err
	}
	if filepath.Dir(clean) != confDDir {
		return "", fmt.Errorf("invalid target file")
	}
	name := filepath.Base(clean)
	if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
		return "", fmt.Errorf("invalid target file")
	}
	return clean, nil
}

// Files describes the config.yaml/conf.d layout, for the target-file picker
// in the host/category forms.
type Files struct {
	ConfigPath string   `json:"configPath"`
	ConfDDir   string   `json:"confDDir"`
	ConfDFiles []string `json:"confDFiles"`
}

// GetFiles reports the config.yaml/conf.d layout.
func GetFiles() (Files, error) {
	configPath, err := config.GetConfigPath()
	if err != nil {
		return Files{}, err
	}
	confDDir, err := config.GetConfDDir()
	if err != nil {
		return Files{}, err
	}
	confDFiles, err := config.ListConfDFiles()
	if err != nil {
		return Files{}, err
	}
	return Files{ConfigPath: configPath, ConfDDir: confDDir, ConfDFiles: confDFiles}, nil
}
