// Package configx wraps go-ssh's config and password packages for use by the
// desktop app. It never forks their logic - it only adds GUI-specific
// conveniences (structured connection fields, stable IDs, jump-chain
// resolution) on top of the shared ~/.go-ssh/ store the CLI also reads.
package configx

import (
	"go-ssh/config"
	"go-ssh/password"
)

// LoadConfig loads the shared ~/.go-ssh/config.yaml + conf.d tree, exactly as
// the go-ssh CLI does.
func LoadConfig() (*config.Config, error) {
	return config.LoadConfig()
}

// NewPasswordStore opens a handle to the shared ~/.go-ssh/passwords.enc
// store, exactly as the go-ssh CLI does. The GUI never calls
// password.PromptMasterPassword (terminal-only); it collects the master
// password through its own unlock screen instead.
func NewPasswordStore() *password.PasswordStore {
	return password.NewPasswordStore()
}

// Summary is a lightweight readout of the shared config tree, used by the
// desktop app's bootstrap screen to prove it can read the same
// ~/.go-ssh/config.yaml + conf.d tree the CLI does.
type Summary struct {
	ConfigPath    string `json:"configPath"`
	CategoryCount int    `json:"categoryCount"`
	HostCount     int    `json:"hostCount"`
	StoreExists   bool   `json:"storeExists"`
}

// LoadSummary loads the shared config and password store and reports their
// current state.
func LoadSummary() (Summary, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Summary{}, err
	}

	path, err := config.GetConfigPath()
	if err != nil {
		return Summary{}, err
	}

	categories, hosts := countTree(cfg.Categories)

	return Summary{
		ConfigPath:    path,
		CategoryCount: categories,
		HostCount:     hosts,
		StoreExists:   NewPasswordStore().StoreExists(),
	}, nil
}

func countTree(categories []config.Category) (categoryCount, hostCount int) {
	for _, cat := range categories {
		categoryCount++
		hostCount += len(cat.Hosts)
		childCategories, childHosts := countTree(cat.Categories)
		categoryCount += childCategories
		hostCount += childHosts
	}
	return categoryCount, hostCount
}
