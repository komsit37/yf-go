# yf-go

[![CI](https://github.com/komsit37/yf-go/actions/workflows/ci.yml/badge.svg)](https://github.com/komsit37/yf-go/actions/workflows/ci.yml)

Small Yahoo Finance tool for Go with two uses:
- CLI `yf` for prices and quote summaries
- Library `github.com/komsit37/yf-go` (`yfgo`) for a reusable client

## Usage

### 1. CLI: run the `yf` command for quick queries.

#### Build
- From source: `go build -o yf ./cmd/yf`
- With Make: `make build`

#### Examples
- Price (table): `./yf price AAPL,TSLA,NVDA,GOOGL,META,MSFT`
![Price table screenshot showing multiple symbols](refs/screenshot.png)

- Quote summary (JSON pretty): `./yf qs AAPL -o json --pretty`
- Quote summary (table) with modules: `./yf qs AAPL -m assetProfile -o table`
- List supported [modules](quotesummary_modules.go): `./yf qs --list-modules`

### 2. Library: import `github.com/komsit37/yf-go` (package `yfgo`) in your Go code.

Example (library):

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/komsit37/yf-go"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    c := yfgo.NewClient()

    // Example 1: v7 quote for multiple symbols
    quotes, err := c.Quote(ctx, []string{"AAPL", "TSLA"})
    if err != nil {
        panic(err)
    }
    for _, q := range quotes {
        if q.RegularMarketPrice != nil {
            fmt.Printf("%s: %.2f\n", q.Symbol, *q.RegularMarketPrice)
        } else {
            fmt.Printf("%s: n/a\n", q.Symbol)
        }
    }

    // Example 2: v10 quoteSummary typed view
    ts, err := c.QuoteSummaryTyped(ctx, "AAPL", []yfgo.QuoteSummaryModule{yfgo.ModulePrice, yfgo.ModuleSummaryDetail})
    if err != nil {
        panic(err)
    }
    if ts.Price != nil {
        fmt.Printf("AAPL prev close: %s\n", ts.Price.RegularMarketPreviousClose.Fmt)
    }
}
```

## Development
- Format: `make fmt` (Go’s canonical tabs; enforced by pre-commit and CI)
- Imports: `make imports` (requires `goimports`; `go install golang.org/x/tools/cmd/goimports@latest`)
- Verify: `make check` (fmt/imports/vet)
- Tests: `go test ./... -race -v`
- Optional pre-commit hook:
  - `git config core.hooksPath .githooks && chmod +x .githooks/pre-commit`

## Project Layout
- `cmd/yf/main.go` — CLI entrypoint
- root package (`github.com/komsit37/yf-go`) — Yahoo Finance client and types (domain logic)
- `refs/` — Upstream references/fixtures (not part of the build)
