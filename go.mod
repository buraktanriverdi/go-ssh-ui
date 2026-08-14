module go-ssh-ui

go 1.25.5

require (
	github.com/creack/pty v1.1.24
	github.com/google/uuid v1.6.0
	github.com/skeema/knownhosts v1.3.2
	github.com/wailsapp/wails/v3 v3.0.0-beta.8
	go-ssh v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.53.0
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/term v0.44.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go-ssh => ../go-ssh
