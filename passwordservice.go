package main

import (
	"fmt"
	"strings"
	"sync"

	"go-ssh/password"
	"go-ssh-ui/internal/configx"
)

// PasswordService is the Wails-bound entry point for the shared
// ~/.go-ssh/passwords.enc store. Unlike go-ssh's web UI, creating and
// unlocking the store both happen right here in the app: the concern that
// kept go-ssh's CLI insisting on a terminal for master-password
// creation/rotation was that the web UI is an HTTP server reachable (even if
// only on loopback) by anything else running as the same user. This service
// has no listening socket - the webview only ever talks to it through
// Wails' own in-process bridge - so that concern doesn't apply, and a
// native-app "type your master password into the app" flow is standard
// (matches how any desktop password manager works).
type PasswordService struct {
	mu             sync.Mutex
	store          *password.PasswordStore
	masterPassword string
	unlocked       bool
}

// PasswordStatus reports whether a store exists on disk and whether this
// session currently has it unlocked.
type PasswordStatus struct {
	StoreExists bool `json:"storeExists"`
	Unlocked    bool `json:"unlocked"`
}

func (s *PasswordService) Status() PasswordStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return PasswordStatus{
		StoreExists: configx.NewPasswordStore().StoreExists(),
		Unlocked:    s.unlocked,
	}
}

// CreateStore initializes a brand-new password store. Only valid when none
// exists yet.
func (s *PasswordService) CreateStore(masterPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(masterPassword) < 8 {
		return fmt.Errorf("master password must be at least 8 characters")
	}
	store := configx.NewPasswordStore()
	if store.StoreExists() {
		return fmt.Errorf("a password store already exists")
	}
	if err := store.Initialize(masterPassword); err != nil {
		return err
	}
	s.store = store
	s.masterPassword = masterPassword
	s.unlocked = true
	return nil
}

// Unlock decrypts the existing store with the given master password.
func (s *PasswordService) Unlock(masterPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if masterPassword == "" {
		return fmt.Errorf("master password is required")
	}
	store := configx.NewPasswordStore()
	if !store.StoreExists() {
		return fmt.Errorf("no password store exists yet")
	}
	if err := store.Load(masterPassword); err != nil {
		return fmt.Errorf("incorrect master password")
	}
	s.store = store
	s.masterPassword = masterPassword
	s.unlocked = true
	return nil
}

// Lock drops the in-memory decrypted store and master password.
func (s *PasswordService) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = nil
	s.masterPassword = ""
	s.unlocked = false
}

func (s *PasswordService) requireUnlockedLocked() error {
	if !s.unlocked || s.store == nil {
		return fmt.Errorf("locked - unlock with the master password first")
	}
	return nil
}

// List returns every stored entry with its password masked.
func (s *PasswordService) List() ([]*password.PasswordEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireUnlockedLocked(); err != nil {
		return nil, err
	}
	return s.store.List(), nil
}

// Add creates a new password entry and persists the store.
func (s *PasswordService) Add(id, description, pw string) ([]*password.PasswordEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireUnlockedLocked(); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" || pw == "" {
		return nil, fmt.Errorf("id and password are required")
	}
	if err := s.store.Add(id, description, pw); err != nil {
		return nil, err
	}
	if err := s.store.Save(s.masterPassword, nil); err != nil {
		return nil, err
	}
	return s.store.List(), nil
}

// Edit updates an existing password entry and persists the store.
func (s *PasswordService) Edit(id, description, pw string) ([]*password.PasswordEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireUnlockedLocked(); err != nil {
		return nil, err
	}
	if pw == "" {
		return nil, fmt.Errorf("password is required")
	}
	if err := s.store.Update(id, description, pw); err != nil {
		return nil, err
	}
	if err := s.store.Save(s.masterPassword, nil); err != nil {
		return nil, err
	}
	return s.store.List(), nil
}

// Delete removes a password entry and persists the store.
func (s *PasswordService) Delete(id string) ([]*password.PasswordEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireUnlockedLocked(); err != nil {
		return nil, err
	}
	if err := s.store.Remove(id); err != nil {
		return nil, err
	}
	if err := s.store.Save(s.masterPassword, nil); err != nil {
		return nil, err
	}
	return s.store.List(), nil
}

// Reveal returns the plaintext password for an entry.
func (s *PasswordService) Reveal(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireUnlockedLocked(); err != nil {
		return "", err
	}
	return s.store.Get(id)
}

// ChangeMasterPassword rotates the master password in place.
func (s *PasswordService) ChangeMasterPassword(oldPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireUnlockedLocked(); err != nil {
		return err
	}
	if len(newPassword) < 8 {
		return fmt.Errorf("new master password must be at least 8 characters")
	}
	if err := s.store.ChangeMasterPassword(oldPassword, newPassword); err != nil {
		return err
	}
	s.masterPassword = newPassword
	return nil
}
