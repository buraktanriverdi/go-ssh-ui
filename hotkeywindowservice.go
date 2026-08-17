package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// HotkeyWindowService drives the iTerm-style "hotkey window": a dedicated
// floating local-shell window (see main.go's second application.Window.
// NewWithOptions call) that a user-defined global shortcut shows/hides
// on demand, from anywhere, even while go-ssh-ui isn't the focused app.
//
// The window itself is created once, hidden, at startup - it can't be
// created here because Wails windows are made against the *application.App
// returned by application.New(), which main() owns. attach hands this
// service both windows right after.
//
// Because the hotkey window is only ever Hide()'d (native orderOut:, never a
// real Close()), it keeps counting as an open NSWindow even after the user
// closes the main window - so the app keeps running and the shortcut keeps
// working until the user actually quits (Cmd+Q / Dock > Quit), which is
// exactly the "always available" behaviour a hotkey window needs.
type HotkeyWindowService struct {
	mainWin   *application.WebviewWindow
	hotkeyWin *application.WebviewWindow
	settings  HotkeyWindowSettings

	// wasMainFocused records, at the moment the hotkey window was last
	// shown, whether go-ssh-ui's own main window had focus - see toggle's
	// hide branch for why.
	wasMainFocused bool

	// resizeMu/resizeTimer debounce WindowDidResize (see attach) into a
	// single persisted write per drag.
	resizeMu    sync.Mutex
	resizeTimer *time.Timer
}

func NewHotkeyWindowService() *HotkeyWindowService {
	return &HotkeyWindowService{settings: loadHotkeyWindowPrefs()}
}

// attach hands the service both windows and, if the persisted settings have
// the feature enabled with a shortcut chosen, registers it with the OS. Also
// wires up height persistence: the window is full-width/top-docked with its
// width locked (see positionOnCursorScreen), so the only thing a drag-resize
// can change is height. macOS has no discrete "resize ended" event through
// Wails (events.Windows.WindowEndResize exists only on Windows) -
// WindowDidResize fires continuously while the user drags, so this
// debounces it into a single persisted write once ~300ms passes with no
// further resize event, i.e. the user let go.
func (h *HotkeyWindowService) attach(mainWin, hotkeyWin *application.WebviewWindow) {
	h.mainWin = mainWin
	h.hotkeyWin = hotkeyWin
	hotkeyWin.OnWindowEvent(events.Common.WindowDidResize, func(event *application.WindowEvent) {
		h.resizeMu.Lock()
		defer h.resizeMu.Unlock()
		if h.resizeTimer != nil {
			h.resizeTimer.Stop()
		}
		h.resizeTimer = time.AfterFunc(300*time.Millisecond, func() {
			_, height := hotkeyWin.Size()
			h.saveHeight(height)
		})
	})
	if h.settings.Enabled && h.settings.Shortcut != "" {
		if err := application.Get().GlobalShortcut.Register(h.settings.Shortcut, h.toggle); err != nil {
			log.Printf("hotkey window: could not register saved shortcut %q: %v", h.settings.Shortcut, err)
		}
	}
}

func (h *HotkeyWindowService) toggle() {
	if h.hotkeyWin == nil {
		return
	}
	if h.hotkeyWin.IsVisible() {
		h.hotkeyWin.Hide()
		if !h.wasMainFocused {
			// The hotkey window was invoked while some other app had focus,
			// not our own main window. Left alone, AppKit promotes the next
			// window of *this* app (the main window) to key status the
			// moment the hotkey window is ordered out - which is exactly
			// the "main window steals focus" behaviour that's wrong here.
			// application.Hide (NSApplication hide:, the same thing Cmd+H
			// does) hides this app's windows without closing them and hands
			// focus back to whatever was frontmost before - i.e. "keep
			// going with whichever window was already open", just like a
			// real iTerm-style hotkey window does.
			application.Get().Hide()
		}
		return
	}

	h.wasMainFocused = h.mainWin != nil && h.mainWin.IsFocused()

	h.hotkeyWin.Show()
	h.positionOnCursorScreen()
	h.hotkeyWin.Focus()
}

// positionOnCursorScreen docks the hotkey window to the top edge, full
// width, of whichever display currently has the mouse cursor - the same
// "appears wherever you're working" behaviour as iTerm's hotkey window.
// Width is locked (Min == Max == the target width) so the only edge left
// for the user to drag-resize is the bottom one; height starts at whatever
// was last dragged (persisted via WindowDidResize in attach), clamped so it
// can never be dragged down over the Dock.
//
// X/width come from WorkArea, not Bounds: WorkArea already excludes
// whichever edge the Dock actually reserves (bottom normally, left/right if
// the user moved it), so this only differs from full screen-edge-to-edge in
// the (uncommon) side-Dock case and is a no-op otherwise. Y stays at
// Bounds.Y (the literal screen top, not WorkArea.Y) to keep the "flush in
// the top corner" look even though that's technically behind the menu bar -
// only the *bottom* edge (where the Dock actually lives) is clamped, via
// maxHeight below.
func (h *HotkeyWindowService) positionOnCursorScreen() {
	if h.hotkeyWin == nil {
		return
	}
	sm := application.Get().Screen
	screen := sm.ScreenNearestDipPoint(currentCursorScreenPoint())
	if screen == nil {
		screen = sm.GetPrimary()
	}
	if screen == nil {
		return
	}
	b := screen.Bounds
	wa := screen.WorkArea

	// How far this window's fixed top (Bounds.Y) can extend down before
	// reaching the Dock's reserved area - not just the screen's bottom edge.
	maxHeight := (wa.Y + wa.Height) - b.Y
	if maxHeight < minHotkeyHeight {
		maxHeight = minHotkeyHeight
	}

	height := h.settings.Height
	if height < minHotkeyHeight {
		height = defaultHotkeyHeight
	}
	if height > maxHeight {
		height = maxHeight
	}

	h.hotkeyWin.SetMinSize(wa.Width, minHotkeyHeight)
	h.hotkeyWin.SetMaxSize(wa.Width, maxHeight)
	h.hotkeyWin.SetSize(wa.Width, height)
	h.hotkeyWin.SetPosition(wa.X, b.Y)
}

// saveHeight persists a user-dragged height, skipping SetSettings (and the
// shortcut re-registration / event broadcast it does) entirely - this fires
// on every resize-drag, and none of that applies here.
func (h *HotkeyWindowService) saveHeight(height int) {
	if height <= 0 || height == h.settings.Height {
		return
	}
	h.settings.Height = height
	_ = saveHotkeyWindowPrefs(h.settings)
}

// GetSettings reports the persisted hotkey window configuration.
func (h *HotkeyWindowService) GetSettings() HotkeyWindowSettings {
	return h.settings
}

// SetSettings persists new hotkey window configuration. BgColor/Opacity/
// Backdrop changes are broadcast as a "hotkey:settings-changed" event
// (registered in main.go) so the hotkey window's own App.svelte instance -
// which may not be the window this call came from, since Settings is
// reachable from either window now - can re-apply its CSS tint live;
// Backdrop itself still only takes visible effect on the next launch, same
// restart caveat as the main window's (Wails picks a window's material at
// creation time, no runtime setter). The shortcut binding, unlike either of
// those, really is swapped out (old one released, new one registered) on
// the spot, no restart or event needed.
func (h *HotkeyWindowService) SetSettings(next HotkeyWindowSettings) error {
	if !next.valid() {
		return fmt.Errorf("geçersiz ayar")
	}
	// Height is drag-persisted only (see saveHeight) - never overwritten by
	// a Settings form round-trip, so a stale value the frontend happened to
	// be holding can't clobber a resize that happened elsewhere meanwhile.
	next.Height = h.settings.Height

	if app := application.Get(); app != nil {
		// Unconditionally release the previous binding (if any) before
		// applying the new one - simpler and just as correct as trying to
		// special-case "only the shortcut string changed" vs. "only enabled
		// flag changed".
		if h.settings.Shortcut != "" {
			_ = app.GlobalShortcut.Unregister(h.settings.Shortcut)
		}
		if next.Enabled && next.Shortcut != "" {
			if err := app.GlobalShortcut.Register(next.Shortcut, h.toggle); err != nil {
				// Roll back so the user isn't left with no shortcut bound at
				// all after a failed change.
				if h.settings.Enabled && h.settings.Shortcut != "" {
					_ = app.GlobalShortcut.Register(h.settings.Shortcut, h.toggle)
				}
				return fmt.Errorf("kısayol kaydedilemedi (başka bir uygulama tarafından kullanılıyor olabilir): %w", err)
			}
		}
	}

	h.settings = next
	if err := saveHotkeyWindowPrefs(next); err != nil {
		return err
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("hotkey:settings-changed", next)
	}
	return nil
}

// Toggle shows/hides the hotkey window on demand - used by Settings' "Test"
// button so appearance/position changes can be previewed without leaving
// the keyboard to go press the actual global shortcut.
func (h *HotkeyWindowService) Toggle() {
	h.toggle()
}
