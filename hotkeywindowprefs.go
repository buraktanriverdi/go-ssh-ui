package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

// HotkeyWindowSettings is the persisted configuration for go-ssh-ui's
// iTerm-style "hotkey window": a dedicated floating local-shell window that
// a global keyboard shortcut shows/hides instantly, even while go-ssh-ui
// isn't the focused app. BgColor/Opacity are applied by the frontend as
// plain CSS (see App.svelte's hotkeyMode handling) painted on top of
// whatever native Backdrop material is chosen - the same
// CSS-tint-over-native-blur split the main window uses (windowprefs.go/
// AppearanceService), just with this window's own settings instead of
// --app-bg. Backdrop itself is still restart-required like the main
// window's, since Wails only picks a window's backdrop material at
// creation time.
type HotkeyWindowSettings struct {
	Enabled bool `json:"enabled"`
	// Shortcut is a Wails accelerator string (e.g. "Cmd+Option+Space").
	// Empty when the user hasn't picked one yet - Enabled with an empty
	// Shortcut just means nothing is registered with the OS.
	Shortcut string `json:"shortcut"`
	// BgColor is a "#rrggbb" hex string for the hotkey window's background
	// tint.
	BgColor string `json:"bgColor"`
	// Opacity is 0-100, how much of BgColor shows over the native Backdrop.
	Opacity int `json:"opacity"`
	// Backdrop picks the native macOS material behind that CSS tint - the
	// same three choices and the same "next launch" caveat as the main
	// window's (see WindowBackdrop in windowprefs.go, reused here as-is).
	Backdrop WindowBackdrop `json:"backdrop"`
	// Height is the user's last dragged height, in points, for the
	// top-docked/full-width window (see HotkeyWindowService.positionOnCursorScreen)
	// - width is never stored, since it's always locked to the target
	// screen's full width at show time, not something the user can pick.
	Height int `json:"height"`
}

const (
	defaultHotkeyBgColor = "#14141a"
	defaultHotkeyOpacity = 90
	defaultHotkeyHeight  = 420
	minHotkeyHeight      = 160
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func defaultHotkeySettings() HotkeyWindowSettings {
	return HotkeyWindowSettings{
		Enabled:  false,
		Shortcut: "",
		BgColor:  defaultHotkeyBgColor,
		Opacity:  defaultHotkeyOpacity,
		Backdrop: BackdropTranslucent,
		Height:   defaultHotkeyHeight,
	}
}

func (s HotkeyWindowSettings) valid() bool {
	return s.Opacity >= 0 && s.Opacity <= 100 && hexColorPattern.MatchString(s.BgColor) && s.Height >= 0 && s.Backdrop.valid()
}

func hotkeyWindowPrefsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "go-ssh-ui", "hotkey.json"), nil
}

// loadHotkeyWindowPrefs defaults to a disabled hotkey window whenever no
// prefs file exists yet or it can't be read - the feature stays fully
// opt-in rather than silently claiming a global shortcut the first time
// this ships.
func loadHotkeyWindowPrefs() HotkeyWindowSettings {
	prefs := defaultHotkeySettings()
	path, err := hotkeyWindowPrefsPath()
	if err != nil {
		return prefs
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return prefs
	}
	var loaded HotkeyWindowSettings
	if err := json.Unmarshal(data, &loaded); err != nil {
		return prefs
	}
	// Backdrop was added after Enabled/Shortcut/BgColor/Opacity/Height - a
	// prefs file written by that earlier version unmarshals with Backdrop at
	// its Go zero value (""), which .valid() rejects. Backfill it instead of
	// discarding the whole file over one missing field: that would silently
	// reset the user's actual saved shortcut/color back to defaults.
	if loaded.Backdrop == "" {
		loaded.Backdrop = BackdropTranslucent
	}
	if !loaded.valid() {
		return prefs
	}
	return loaded
}

func saveHotkeyWindowPrefs(s HotkeyWindowSettings) error {
	path, err := hotkeyWindowPrefsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
