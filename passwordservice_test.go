package main

import "testing"

// t.Setenv("HOME", t.TempDir()) redirects go-ssh/password's os.UserHomeDir()
// lookups to an isolated directory, so these tests never touch a real
// ~/.go-ssh/passwords.enc.

func TestPasswordServiceLifecycle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &PasswordService{}

	status := svc.Status()
	if status.StoreExists || status.Unlocked {
		t.Fatalf("expected fresh status, got %+v", status)
	}

	if err := svc.CreateStore("short"); err == nil {
		t.Fatalf("expected error for short master password")
	}

	if err := svc.CreateStore("correct horse battery staple"); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}

	status = svc.Status()
	if !status.StoreExists || !status.Unlocked {
		t.Fatalf("expected unlocked store after create, got %+v", status)
	}

	entries, err := svc.Add("prod-db", "prod database", "s3cr3t")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "prod-db" {
		t.Fatalf("unexpected entries after Add: %+v", entries)
	}

	revealed, err := svc.Reveal("prod-db")
	if err != nil || revealed != "s3cr3t" {
		t.Fatalf("Reveal mismatch: %q err=%v", revealed, err)
	}

	svc.Lock()
	status = svc.Status()
	if !status.StoreExists || status.Unlocked {
		t.Fatalf("expected locked-but-existing after Lock, got %+v", status)
	}

	if _, err := svc.List(); err == nil {
		t.Fatalf("expected error listing while locked")
	}

	if err := svc.Unlock("wrong password"); err == nil {
		t.Fatalf("expected error unlocking with wrong password")
	}

	if err := svc.Unlock("correct horse battery staple"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	entries, err = svc.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("List after unlock: %+v err=%v", entries, err)
	}

	if _, err := svc.Delete("prod-db"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
