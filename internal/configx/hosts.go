package configx

import (
	"fmt"
	"strings"

	"go-ssh/config"

	"github.com/google/uuid"
)

// validAuthMethods are the AuthMethod values the desktop app's SSH engine
// understands. An empty value means "not configured yet" (advanced/legacy
// mode, connecting via Command/Commands instead).
var validAuthMethods = map[string]bool{
	"":         true,
	"password": true,
	"key":      true,
	"agent":    true,
}

// NormalizeHost trims and validates a host coming from the frontend, and
// assigns a stable ID if it doesn't have one yet (new host). Mirrors go-ssh's
// web/handlers_config.go normalizeHost (trim name/description, keep exactly
// one of Command/Commands populated, drop blank command-step lines), extended
// with the structured connection fields go-ssh-ui adds on top of go-ssh's
// schema.
func NormalizeHost(h config.Host) (config.Host, error) {
	name := strings.TrimSpace(h.Name)
	if name == "" {
		return config.Host{}, fmt.Errorf("host name is required")
	}

	authMethod := strings.TrimSpace(h.AuthMethod)
	if !validAuthMethods[authMethod] {
		return config.Host{}, fmt.Errorf("invalid auth method: %s", authMethod)
	}

	id := strings.TrimSpace(h.ID)
	if id == "" {
		id = uuid.NewString()
	}

	host := config.Host{
		ID:           id,
		Name:         name,
		Description:  strings.TrimSpace(h.Description),
		HostAddr:     strings.TrimSpace(h.HostAddr),
		Port:         h.Port,
		User:         strings.TrimSpace(h.User),
		AuthMethod:   authMethod,
		IdentityFile: strings.TrimSpace(h.IdentityFile),
		PasswordID:   strings.TrimSpace(h.PasswordID),
		JumpVia:      trimNonEmpty(h.JumpVia),
	}

	if len(h.Commands) > 0 {
		host.Commands = trimNonEmpty(h.Commands)
	} else {
		host.Command = strings.TrimSpace(h.Command)
	}

	return host, nil
}

// NormalizeCategory trims a category coming from the frontend.
func NormalizeCategory(c config.Category) (config.Category, error) {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		return config.Category{}, fmt.Errorf("category name is required")
	}
	return config.Category{Name: name, Description: strings.TrimSpace(c.Description)}, nil
}

func trimNonEmpty(lines []string) []string {
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
