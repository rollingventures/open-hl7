#!/bin/sh
# Compiles the sample WASM transform guest to internal/transform/assets/transform.wasm.
# Requires Go 1.24+ (for //go:wasmexport reactor modules).
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(git -C "$HERE" rev-parse --show-toplevel)"
mkdir -p "$HERE/assets"
cd "$ROOT/examples/wasm-transform/guest"
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o "$HERE/assets/transform.wasm" .
echo "built $HERE/assets/transform.wasm ($(wc -c < "$HERE/assets/transform.wasm") bytes)"
