package main

import "fmt"

// AppearanceService exposes the one appearance preference that can't be a
// plain frontend/localStorage value (see windowprefs.go): which native
// macOS window backdrop material is used. Changing it can't take effect
// live like the rest of Settings does - Wails has no runtime setter for a
// window's backdrop material, only a creation-time option - so SetBackdrop
// just persists the choice for main() to pick up on the next launch, and
// the frontend is responsible for telling the user to restart.
type AppearanceService struct{}

// GetBackdrop reports the currently persisted backdrop choice: "translucent"
// (blurred vibrancy, the default), "transparent" (crisp, unblurred), or
// "liquidGlass" (Apple's macOS 15+ Liquid Glass material).
func (a *AppearanceService) GetBackdrop() string {
	return string(loadWindowPrefs().Backdrop)
}

// SetBackdrop persists the preference for the next launch.
func (a *AppearanceService) SetBackdrop(backdrop string) error {
	b := WindowBackdrop(backdrop)
	if !b.valid() {
		return fmt.Errorf("unknown backdrop %q", backdrop)
	}
	return saveWindowPrefs(windowPrefs{Backdrop: b})
}
