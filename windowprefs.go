package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// WindowBackdrop is the persisted choice for the native macOS window
// background - see AppearanceService for why this can't just be a live CSS
// value like the rest of Settings.
type WindowBackdrop string

const (
	// BackdropTranslucent is the frosted/blurred macOS vibrancy look this
	// app originally launched with - the default.
	BackdropTranslucent WindowBackdrop = "translucent"
	// BackdropTransparent is just as see-through as Translucent (the CSS
	// side's --app-bg-alpha still controls how much shows through) but
	// crisp, with no blur at all.
	BackdropTransparent WindowBackdrop = "transparent"
	// BackdropLiquidGlass is Apple's macOS 15+ Liquid Glass material; Wails
	// falls back to BackdropTranslucent by itself on older macOS.
	BackdropLiquidGlass WindowBackdrop = "liquidGlass"
)

func (b WindowBackdrop) valid() bool {
	switch b {
	case BackdropTranslucent, BackdropTransparent, BackdropLiquidGlass:
		return true
	default:
		return false
	}
}

// macBackdrop maps a persisted WindowBackdrop choice to the Wails option
// that actually creates it - shared by both windows (main.go), since both
// now offer the same three-way Bulanık/Liquid Glass/Şeffaf picker (see
// AppearanceService for the main window, HotkeyWindowService for the
// hotkey one), just backed by separate persisted preferences.
func macBackdrop(b WindowBackdrop) application.MacBackdrop {
	switch b {
	case BackdropTransparent:
		return application.MacBackdropTransparent
	case BackdropLiquidGlass:
		return application.MacBackdropLiquidGlass
	default:
		return application.MacBackdropTranslucent
	}
}

// windowPrefs holds the one appearance preference that must be known
// *before* the native window exists. Everything else in Settings
// (frontend/src/lib/appearance.svelte.ts - background opacity, .glass-panel
// blur radius, accent color) lives in the webview's own localStorage and
// applies live, but there's no webview yet at the point main() needs this
// one, so it's persisted to its own small file instead. Kept out of
// ~/.go-ssh/ since that directory's schema is shared with the go-ssh CLI -
// this is UI-only state.
type windowPrefs struct {
	Backdrop WindowBackdrop `json:"backdrop"`
}

func windowPrefsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "go-ssh-ui", "window.json"), nil
}

// loadWindowPrefs defaults to the original blurred-vibrancy look whenever no
// prefs file exists yet, it can't be read, or it holds an unrecognized value
// (e.g. an older "nativeBlur" boolean from before this became a 3-way choice).
func loadWindowPrefs() windowPrefs {
	prefs := windowPrefs{Backdrop: BackdropTranslucent}
	path, err := windowPrefsPath()
	if err != nil {
		return prefs
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return prefs
	}
	_ = json.Unmarshal(data, &prefs)
	if !prefs.Backdrop.valid() {
		prefs.Backdrop = BackdropTranslucent
	}
	return prefs
}

func saveWindowPrefs(p windowPrefs) error {
	path, err := windowPrefsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
