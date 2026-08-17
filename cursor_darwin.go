//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

typedef struct {
	double x;
	double y;
} CursorPoint;

// currentCursorPositionTopLeft returns the current mouse position in the
// same coordinate space Wails' own ScreenManager uses for Screen.Bounds
// (see wails' screen_darwin.go): Y-down, origin at the top-left of the
// *primary* screen (NSScreen.screens[0], not NSScreen.mainScreen - Wails
// deliberately uses the former for this flip, so this matches it exactly).
// NSEvent.mouseLocation itself is Y-up with origin at the primary screen's
// bottom-left, hence the flip.
static CursorPoint currentCursorPositionTopLeft(void) {
	NSPoint p = [NSEvent mouseLocation];
	NSScreen *primary = [[NSScreen screens] firstObject];
	if (primary == NULL) {
		primary = [NSScreen mainScreen];
	}
	double primaryHeight = primary != NULL ? primary.frame.size.height : 0;

	CursorPoint result;
	result.x = p.x;
	result.y = primaryHeight - p.y;
	return result;
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// currentCursorScreenPoint reports the mouse cursor's current position, in
// the coordinate space application.Screen.Bounds and
// ScreenManager.ScreenNearestDipPoint use - so its result can be passed
// straight into ScreenNearestDipPoint to find which display the pointer is
// on right now. Wails has no public API for this (only window/screen
// geometry, not the live cursor), hence the small direct AppKit call.
func currentCursorScreenPoint() application.Point {
	p := C.currentCursorPositionTopLeft()
	return application.Point{X: int(p.x), Y: int(p.y)}
}
