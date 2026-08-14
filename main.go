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

	app := application.New(application.Options{
		Name:        "go-ssh-ui",
		Description: "Masaüstü SSH host yöneticisi ve terminal istemcisi",
		Services: []application.Service{
			application.NewService(&ConfigService{}),
			application.NewService(hostService),
			application.NewService(passwordService),
			application.NewService(terminalService),
			application.NewService(fileService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	application.RegisterEvent[FilesDroppedEvent]("files:dropped")

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
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// Forwards native Finder drops to the frontend - see FilesDroppedEvent.
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

	// Run the application. This blocks until the application has been exited.
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
