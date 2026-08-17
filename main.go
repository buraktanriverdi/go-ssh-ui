package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// FilesDroppedEvent is emitted to the frontend when the user drags files
// from Finder onto a `data-file-drop-target` element - id matches that
// element's DOM id, so a FilesPane with several instances mounted at once
// (see App.svelte's stay-mounted-hidden tab pattern) can tell whether a drop
// was meant for it.
type FilesDroppedEvent struct {
	ElementID string   `json:"elementId"`
	Files     []string `json:"files"`
}

// wireFileDrop forwards native Finder drops on win to the frontend as a
// FilesDroppedEvent. Needed on both windows now that the hotkey window
// mounts the same <App/> (and therefore the same FilesPane) as the main
// one - each window needs its own EnableFileDrop + this handler, since
// OnWindowEvent only fires for drops that landed on that specific window.
func wireFileDrop(win *application.WebviewWindow) {
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		details := event.Context().DropTargetDetails()
		if details == nil {
			return
		}
		application.Get().Event.Emit("files:dropped", FilesDroppedEvent{
			ElementID: details.ElementID,
			Files:     event.Context().DroppedFiles(),
		})
	})
}

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 'Services' are the Go struct instances the frontend can call methods
	// on. ConfigService is a read-only bootstrap summary; HostService and
	// PasswordService drive the host tree and the shared password store,
	// both reading/writing the same ~/.go-ssh/ directory as the go-ssh CLI.
	// TerminalService and FileService are constructed explicitly (not just
	// `&TerminalService{}`) because both need the same sshengine.Manager -
	// sharing it means a terminal tab and a file-manager tab open on the
	// same structured host reuse one pooled *ssh.Client instead of dialing
	// twice.
	hostService := &HostService{}
	passwordService := &PasswordService{}
	sshManager := NewSSHManager(hostService, passwordService)
	terminalService := NewTerminalService(hostService, sshManager)
	fileService := NewFileService(hostService, sshManager)
	recordService := NewRecordService(sshManager)
	hotkeyWindowService := NewHotkeyWindowService()

	// Read before the window is created - see windowprefs.go for why this
	// one preference can't just live in the webview's localStorage like the
	// rest of Settings.
	winPrefs := loadWindowPrefs()

	app := application.New(application.Options{
		Name:        "go-ssh-ui",
		Description: "Masaüstü SSH host yöneticisi ve terminal istemcisi",
		Services: []application.Service{
			application.NewService(&ConfigService{}),
			application.NewService(hostService),
			application.NewService(passwordService),
			application.NewService(terminalService),
			application.NewService(fileService),
			application.NewService(recordService),
			application.NewService(&AppearanceService{}),
			application.NewService(hotkeyWindowService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	application.RegisterEvent[FilesDroppedEvent]("files:dropped")
	application.RegisterEvent[HotkeyWindowSettings]("hotkey:settings-changed")

	// Create a new window with the necessary options.
	// 'Mac' options give it the translucent, inset-title-bar "liquid glass"
	// look on macOS.
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "go-ssh-ui",
		Width:  1180,
		Height: 760,
		// Lets Finder drags onto `data-file-drop-target` elements (the file
		// manager's upload drop zone) trigger a FilesDropped event with the
		// real filesystem paths - see PLAN.md §5.
		EnableFileDrop: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                macBackdrop(winPrefs.Backdrop),
			TitleBar:                application.MacTitleBarHiddenInset,
			// Only consulted when Backdrop is MacBackdropLiquidGlass.
			// Automatic/Automatic lets AppKit pick light-vs-dark glass and
			// its underlying material itself, matching how the rest of
			// this app's theming already just follows the system appearance.
			LiquidGlass: application.MacLiquidGlass{
				Style:    application.LiquidGlassStyleAutomatic,
				Material: application.NSVisualEffectMaterialAuto,
			},
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	wireFileDrop(win)

	// The "hotkey window" (see HotkeyWindowService/PLAN.md Faz 6): a second,
	// initially hidden window shown/hidden by a global shortcut -
	// go-ssh-ui's answer to iTerm's hotkey window. It's its own window (not
	// a mode of `win`) so it can float AlwaysOnTop, skip the dock-focus
	// dance, and carry its own appearance settings independently of the
	// main window's. Its Backdrop material (blurred/liquid-glass/crisp) is
	// the user's saved hotkey-window choice, same restart caveat as the
	// main window's - BackgroundColour is always fully clear at creation
	// because the visible tint is CSS now (App.svelte's hotkeyMode
	// handling), painted over whatever this Backdrop renders, not a native
	// color. Frameless since a floating quick-access panel has no reason
	// for a title bar or traffic lights. Width/Height here are just a seed
	// for the very first Show() - HotkeyWindowService.positionOnCursorScreen
	// immediately docks it to the top edge, full width, of whichever screen
	// has the cursor, every time it's shown. `?mode=hotkey` tells the
	// frontend (see main.ts/App.svelte) it's running in the hotkey window,
	// which only changes window-chrome details (no sidebar, CSS tint) - the
	// actual content (tabs, Başlangıç) is the same <App/> as the main window.
	hotkeyWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "go-ssh-ui — Hotkey Terminal",
		Width:           900,
		Height:          defaultHotkeyHeight,
		Hidden:          true,
		Frameless:       true,
		AlwaysOnTop:     true,
		HideOnFocusLost: true,
		EnableFileDrop:  true,
		Mac: application.MacWindow{
			Backdrop: macBackdrop(hotkeyWindowService.GetSettings().Backdrop),
			LiquidGlass: application.MacLiquidGlass{
				Style:    application.LiquidGlassStyleAutomatic,
				Material: application.NSVisualEffectMaterialAuto,
			},
		},
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/?mode=hotkey",
	})
	wireFileDrop(hotkeyWin)
	hotkeyWindowService.attach(win, hotkeyWin)

	// Run the application. This blocks until the application has been exited.
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
