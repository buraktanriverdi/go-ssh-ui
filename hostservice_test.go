package main

import (
	"testing"

	"go-ssh/config"
)

// t.Setenv("HOME", t.TempDir()) redirects go-ssh/config's os.UserHomeDir()
// lookups to an isolated directory, so these tests never touch a real
// ~/.go-ssh installation.

func TestHostServiceCRUD(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &HostService{}

	files, err := svc.GetFiles()
	if err != nil {
		t.Fatalf("GetFiles: %v", err)
	}

	cfg, err := svc.AddCategory(AddCategoryRequest{
		TargetFile:  files.ConfigPath,
		Name:        "Prod",
		Description: "Production",
	})
	if err != nil {
		t.Fatalf("AddCategory: %v", err)
	}
	if len(cfg.Categories) != 1 || cfg.Categories[0].Name != "Prod" {
		t.Fatalf("unexpected categories after AddCategory: %+v", cfg.Categories)
	}

	cfg, err = svc.AddHost(AddHostRequest{
		TargetFile:   files.ConfigPath,
		CategoryPath: []string{"Prod"},
		Host:         config.Host{Name: "Web1", HostAddr: "web1.example.com", Port: 22, User: "deploy"},
	})
	if err != nil {
		t.Fatalf("AddHost: %v", err)
	}
	host := cfg.Categories[0].Hosts[0]
	if host.Name != "Web1" || host.HostAddr != "web1.example.com" || host.Port != 22 || host.User != "deploy" {
		t.Fatalf("unexpected host after AddHost: %+v", host)
	}
	if host.ID == "" {
		t.Fatalf("expected host ID to be auto-assigned")
	}
	firstID := host.ID

	editedHost := host
	editedHost.Description = "primary"
	cfg, err = svc.EditHost(EditHostRequest{
		SourceFile:   host.SourceFile,
		CategoryPath: []string{"Prod"},
		OriginalName: "Web1",
		Host:         editedHost,
	})
	if err != nil {
		t.Fatalf("EditHost: %v", err)
	}
	host = cfg.Categories[0].Hosts[0]
	if host.Description != "primary" {
		t.Fatalf("description not updated: %+v", host)
	}
	if host.ID != firstID {
		t.Fatalf("host ID changed on edit: got %s want %s", host.ID, firstID)
	}

	cfg, err = svc.DeleteHost(DeleteHostRequest{
		SourceFile:   host.SourceFile,
		CategoryPath: []string{"Prod"},
		Name:         "Web1",
	})
	if err != nil {
		t.Fatalf("DeleteHost: %v", err)
	}
	if len(cfg.Categories[0].Hosts) != 0 {
		t.Fatalf("expected host removed, got %+v", cfg.Categories[0].Hosts)
	}

	cfg, err = svc.DeleteCategory(DeleteCategoryRequest{
		SourceFile:   cfg.Categories[0].SourceFile,
		CategoryPath: []string{"Prod"},
	})
	if err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	if len(cfg.Categories) != 0 {
		t.Fatalf("expected category removed, got %+v", cfg.Categories)
	}
}

func TestHostServiceRejectsPathOutsideConfDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &HostService{}
	if _, err := svc.AddCategory(AddCategoryRequest{TargetFile: "/etc/passwd", Name: "Evil"}); err == nil {
		t.Fatalf("expected error for out-of-bounds target file")
	}
}
