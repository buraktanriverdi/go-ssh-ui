//go:build !darwin

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// currentCursorScreenPoint has no non-macOS implementation (this app is
// macOS-first per PLAN.md) - callers fall back to the primary screen when
// this returns the zero value and no screen's bounds contain it.
func currentCursorScreenPoint() application.Point {
	return application.Point{}
}
