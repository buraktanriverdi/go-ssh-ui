package main

import "go-ssh-ui/internal/configx"

// ConfigService is the Wails-bound entry point the frontend uses to read the
// config/password state shared with the go-ssh CLI under ~/.go-ssh/.
type ConfigService struct{}

// LoadSummary reports the current shared config tree and password store
// state so the frontend can prove end-to-end that it reads the same
// ~/.go-ssh/ directory the CLI does.
func (c *ConfigService) LoadSummary() (configx.Summary, error) {
	return configx.LoadSummary()
}
