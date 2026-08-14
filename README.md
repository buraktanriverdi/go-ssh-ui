# go-ssh-ui

A desktop SSH host manager and terminal client built with [Wails v3](https://v3.wails.io/) (Go backend + native WebView, Svelte frontend). This is the desktop counterpart of the [`go-ssh`](../go-ssh) CLI/TUI — it reads and writes the same `~/.go-ssh/` config and password store, using the exact same Go code (via a `replace go-ssh => ../go-ssh` directive in `go.mod`).

## Features

- Manage SSH hosts, categories, and passwords from a native GUI
- Tabbed multi-terminal (`xterm.js`), with a dual SSH engine:
  - **subprocess engine** — shells out to the real `ssh` binary, so jump-host chains, `SEND`/`EXPECT` scripted logins, `dzdo`/`sudo` escalation, `~/.ssh/config`, and ssh-agent all work exactly as in the `go-ssh` CLI
  - **native engine** (`golang.org/x/crypto/ssh`) — used for simple, structured hosts; supports pooled/shared connections across tabs
- WinSCP-style drag-and-drop file manager over SCP, including transfers across multi-hop jump-host chains
- Global keyboard shortcut to bring the terminal window to the front
- macOS Liquid Glass transparency
- Optional headless "server mode" (HTTP only, no GUI) with a Docker image, for running on a remote box

## Prerequisites

- **Go** >= 1.25 (see `go.mod`)
- **Node.js + npm** for the Svelte/Vite frontend (bun/pnpm/yarn also work, see `PACKAGE_MANAGER` in `Taskfile.yml`)
- **[Task](https://taskfile.dev/)** — the CLI task runner that drives `dev`/`build`/`package` (see below)
- **Wails3 CLI** — see [Installing Wails3](#installing-wails3) below
- A checkout of the sibling [`go-ssh`](../go-ssh) repo at `../go-ssh` — this repo's `go.mod` uses `replace go-ssh => ../go-ssh` to reuse its config/password packages, so the two repos must sit next to each other:
  ```
  git/
  ├── go-ssh/
  └── go-ssh-ui/
  ```

## Installing Wails3

This project targets **Wails v3** (currently beta), whose CLI binary is `wails3` — a separate install from Wails v2's `wails`. Install it with Go:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Make sure `$(go env GOPATH)/bin` (usually `~/go/bin`) is on your `PATH`, then confirm it's working and check for missing platform dependencies (Xcode Command Line Tools on macOS, `webkit2gtk`/`libgtk-3` on Linux, WebView2 on Windows, etc.):

```bash
wails3 version
wails3 doctor
```

You'll also need the [Task](https://taskfile.dev/) CLI, since `Taskfile.yml` (not `wails3` directly) is what this repo's `dev`/`build`/`package` commands go through:

```bash
go install github.com/go-task/task/v3/cmd/task@latest
```

Full platform-specific instructions: [Wails3 installation guide](https://v3.wails.io/getting-started/installation/) · [Task installation guide](https://taskfile.dev/installation/).

## Getting Started

1. Clone this repo as a sibling of `go-ssh` (see [Prerequisites](#prerequisites)).
2. Install frontend dependencies (also done automatically by `task dev`/`task build`):
   ```bash
   cd frontend && npm install && cd ..
   ```
3. Run in development mode — hot-reloads both the Go backend and the frontend:
   ```bash
   task dev
   ```
4. Build a production binary (output in `bin/`):
   ```bash
   task build
   ```

## Other Tasks

- `task package` — build a distributable package for the current OS (`.app`/`.dmg` on macOS, installer on Windows, etc.)
- `task run` — run a previously built binary
- `task build:server` / `task run:server` — headless HTTP-only mode, no GUI
- `task build:docker` / `task run:docker` — build/run the server-mode Docker image
- `task setup:docker` — pull the cross-compilation Docker image (~800MB), needed to cross-build for other OSes

Cross-compile for another OS with, e.g., `GOOS=windows task build`.

## Testing

```bash
go test ./...
```

## Project Structure

- `frontend/` — Svelte + TypeScript + Vite UI: terminal panes (`xterm.js`), host tree, file manager
- `internal/sshengine/` — the dual SSH engine (subprocess `ssh` via pty, and native `golang.org/x/crypto/ssh`), jump-host chaining, host key handling
- `internal/scpfs/` — hand-rolled SCP wire-protocol client backing the file manager
- `internal/configx/` — config/host lookup helpers shared across services
- `hostservice.go`, `passwordservice.go`, `terminalservice.go`, `fileservice.go`, `configservice.go` — the Wails services exposed to the frontend
- `main.go` — application entry point and service wiring
- `build/` — per-platform Wails build assets (icons, `Info.plist`, platform Taskfiles)
- `PLAN.md` — architecture notes and design decisions (Turkish)

## Configuration

Host, category, and password data lives in `~/.go-ssh/` (`config.yaml`, `conf.d/`, `passwords.enc`) and is shared byte-for-byte with the `go-ssh` CLI — both tools read and write it using the same underlying packages.

## Learn More

- [Wails3 documentation](https://v3.wails.io/)
- [Wails Discord](https://discord.gg/JDdSxwjhGf) / [Wails GitHub discussions](https://github.com/wailsapp/wails/discussions)
