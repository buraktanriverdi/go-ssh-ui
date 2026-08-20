#!/usr/bin/env bash
# Builds go-ssh-ui. Wraps `task <target>` (default: build), auto-installing
# the `task` and `wails3` CLIs if they're missing. Extra args/vars pass
# through, e.g.:
#   ./build.sh                  # task build
#   ./build.sh package          # task package
#   ./build.sh package:dmg      # build a macOS .dmg installer (darwin only)
#   ./build.sh build:server     # headless server build
#   GOOS=windows ./build.sh     # cross-compile
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

GOBIN="$(go env GOPATH 2>/dev/null)/bin"
export PATH="$GOBIN:$PATH"

require() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "error: $1 is required but not found: $2" >&2
        exit 1
    }
}

install_go_tool() {
    local bin_name="$1" pkg="$2"
    if ! command -v "$bin_name" >/dev/null 2>&1; then
        echo "==> $bin_name not found, installing via 'go install $pkg'"
        go install "$pkg"
    fi
}

require go "https://go.dev/dl/"
require npm "https://nodejs.org/en/download/"

install_go_tool task github.com/go-task/task/v3/cmd/task@latest
install_go_tool wails3 github.com/wailsapp/wails/v3/cmd/wails3@latest

TASK_NAME="build"
if [ $# -gt 0 ] && [[ "$1" != *=* ]]; then
    TASK_NAME="$1"
    shift
fi

echo "==> task $TASK_NAME $*"
task "$TASK_NAME" "$@"

echo "==> Done. Binary is in ${SCRIPT_DIR}/bin/"
