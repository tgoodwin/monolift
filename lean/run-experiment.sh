#!/usr/bin/env bash
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd "$root"

lake update
lake build lean-experiment

work=$(mktemp -d "${TMPDIR:-/tmp}/monolift-lean-run.XXXXXX")
work=$(cd -- "$work" && pwd)
trap 'rm -rf "$work"' EXIT
frontend="$work/nativefrontend"
wire="$work/negate.json"

# Build and run the Go tools on the host; only the wire targets linux/amd64.
unset GOOS GOARCH
export GOENV=off GO111MODULE=off GOTOOLCHAIN=go1.26.5
(
  cd .lake/packages/GoLean
  go build -o "$frontend" ./tools/nativefrontend
)
GOOS=linux GOARCH=amd64 "$frontend" --subject --dir testdata/negate --out "$wire"
go run ./testdata/negate/main.go > "$work/go.txt" 2>&1
.lake/build/bin/lean-experiment "$wire" > "$work/lean.txt"
diff -u "$work/go.txt" "$work/lean.txt"

printf 'Go and GoLean agree (input, output, error == nil):\n'
cat "$work/lean.txt"
printf 'Single-attempt checks passed: delivered, request lost, reply lost.\n'
