# Repository Guidelines

## Project Structure & Modules
- `main.go`: CLI entrypoint.
- `cmd/`: Cobra commands (`root.go`, `qs.go`). CLI concerns only.
- `pkg/yfapi/`: Yahoo Finance client and types. Reusable, testable logic.
- `refs/`: Upstream references and fixtures (not part of the build).
- `yf`: Built binary (ignored in Git ideally).

## Build, Test, Run
- Build: `go build -o yf .` — compiles the CLI.
- Run (help): `go run . --help`
- Quote summary (JSON): `./yf qs AAPL -o json --pretty`
- Quote summary (table): `./yf qs AAPL -o table`
- List supported modules: `./yf qs --list-modules`
- Tests: `go test ./... -v` — none yet, but add under `pkg/yfapi`.

## Coding Style & Naming
- Formatter: `go fmt ./...` (tabs, standard Go formatting). Lint with `go vet ./...`.
- Packages: lowercase, short names (e.g., `yfapi`).
- Files: descriptive lowercase; tests end with `_test.go`.
- Exported names: PascalCase with doc comments starting with the identifier name.
- CLI code stays in `cmd/`; domain logic in `pkg/yfapi`.

## Testing Guidelines
- Framework: Go `testing` with table-driven tests.
- Location: co-locate tests next to sources (e.g., `pkg/yfapi/client_test.go`).
- HTTP: use `httptest.Server` and inject via `Client.http` to avoid real network calls.
- Run with race detector locally: `go test ./... -race`.

## Commit & PR Guidelines
- Commits: imperative present (“add”, “fix”, “refactor”); keep subject ≤72 chars.
- Include scope when useful (e.g., `yfapi:` or `cmd/qs:`). Example: `cmd/qs: add --list-modules flag`.
- PRs: include purpose, key changes, how to run (`./yf qs AAPL -o table`), and linked issues.

## Configuration & Security
- Config: Viper reads env with `YF_` prefix and optional `yf.(yaml|json|toml)` in CWD or `YF_CONFIG` path.
  - Examples: `YF_FORMAT=table`, `YF_PRETTY=1`, `YF_CONFIG=./yf.yaml`.
- Networking: client manages cookies/crumbs; avoid adding secrets. Do not commit local binaries.

## Architecture Overview
- Flow: CLI (`cmd/`) parses flags -> calls `pkg/yfapi` -> renders JSON or table.
- Typed view: `QuoteSummaryTyped` provides convenient fields for table output.

