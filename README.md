# yf-go

[![CI](https://github.com/komsit37/yf-go/actions/workflows/ci.yml/badge.svg)](https://github.com/komsit37/yf-go/actions/workflows/ci.yml)

Tiny Go CLI for Yahoo Finance data. The CLI parses flags, calls a reusable client exposed by the root module (`github.com/pkomsit/yf-go`), and renders JSON or a table.

## Requirements
- Go 1.21+

## Build
- From source: `go build -o yf ./cmd/yf`
- With Make: `make build`

## Run
- Help: `go run ./cmd/yf --help`
- Built binary help: `./yf --help`

## Examples
- Quote summary (JSON pretty): `./yf qs AAPL -o json --pretty`
- Quote summary (table): `./yf qs AAPL -o table`
- List supported modules: `./yf qs --list-modules`

## Configuration
Uses Viper with `YF_`-prefixed env vars and optional `yf.(yaml|json|toml)` in CWD or a path from `YF_CONFIG`.
- `YF_FORMAT=table` — default output format
- `YF_PRETTY=1` — pretty-print JSON
- `YF_CONFIG=./yf.yaml` — explicit config file path

## Development
- Format: `make fmt` (Go’s canonical tabs; enforced by pre-commit and CI)
- Imports: `make imports` (requires `goimports`; `go install golang.org/x/tools/cmd/goimports@latest`)
- Verify: `make check` (fmt/imports/vet)
- Tests: `go test ./... -race -v`
- Optional pre-commit hook:
  - `git config core.hooksPath .githooks && chmod +x .githooks/pre-commit`

## Project Layout
- `cmd/yf/main.go` — CLI entrypoint
- `cmd/` — Cobra commands (CLI only)
- root package (`github.com/pkomsit/yf-go`) — Yahoo Finance client and types (domain logic)
- `refs/` — Upstream references/fixtures (not part of the build)
