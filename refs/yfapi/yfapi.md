# Yahoo Finance API Usage (for this project)

This document explains how this repository interacts with Yahoo Finance’s unofficial endpoints — specifically the `quoteSummary` API — including the exact URLs, query parameters, session/cookie handling (crumb), error behavior, and recommended rate‑limit practices.

## Overview

- Primary module: `pkg/yfapi` (reusable client and types)
- Main flow: CLI (`cmd/`) parses flags → calls `pkg/yfapi` → renders JSON/table
- Focused endpoint: `quoteSummary` (v10)
- Session model: cookie jar + “crumb” token

## Endpoints Used

1) Warm up cookies

- URL: `https://fc.yahoo.com`
- Method: `GET`
- Purpose: Seed the cookie jar with Yahoo cookies to make subsequent crumb requests more reliable.

2) Fetch crumb

- URL: `https://query1.finance.yahoo.com/v1/test/getcrumb`
- Method: `GET`
- Returns: A short text token (the “crumb”) used as a query parameter on authenticated finance endpoints.

3) Quote Summary (data)

- URL: `https://query1.finance.yahoo.com/v10/finance/quoteSummary/{symbol}`
- Method: `GET`
- Required query parameters:
  - `modules`: Comma‑separated list of modules to include (see below).
  - `crumb`: The crumb string obtained from the crumb endpoint.

Notes

- The code path escapes `{symbol}` via `url.PathEscape` for safety.
- A browser‑like `User-Agent` and common headers (`Accept`, `Accept-Language`) are sent to reduce blocking.
- Hostname `query1.finance.yahoo.com` is used (HTTP/1.0, widely compatible). `query2` is an alternative with HTTP/1.1; this code uses `query1`.

## Modules

`quoteSummary` supports many sub‑modules; this project exposes them via `pkg/yfapi.AllowedQuoteSummaryModules` (e.g., `price`, `summaryDetail`, `financialData`, `assetProfile`, etc.).

Defaults used by table output (`pkg/yfapi.DefaultQuoteSummaryModules`):

- `price`
- `summaryDetail`
- `financialData`

You may request additional modules by passing `--modules` on the CLI or using the library interface with a custom slice.

## Parameters Sent

- Path parameter: `{symbol}` (URL path‑escaped)
- Query parameters:
  - `modules`: `price,summaryDetail,financialData` (or as specified)
  - `crumb`: The crumb value previously fetched
- Headers:
  - `User-Agent`: A realistic browser UA string
  - `Accept`: `application/json, text/plain, */*`
  - `Accept-Language`: `en-US,en;q=0.9`

Optional Yahoo parameters like `lang` and `region` can be included, but this client relies on defaults and `Accept-Language` for now.

## Session & Crumb Handling

- Cookie storage: `net/http/cookiejar` on the client (`*http.Client.Jar`)
- Crumb lifecycle:
  1. On first use (or when missing), visit `https://fc.yahoo.com` to warm cookies.
  2. Request crumb from `https://query1.finance.yahoo.com/v1/test/getcrumb`.
  3. Cache crumb in the client (`Client.crumb`).
  4. Send crumb as `crumb=...` on `quoteSummary` requests.
  5. If a request fails with 401 or an “invalid crumb” response, clear the cached crumb, fetch a new one once, and retry the request.

This approach mirrors common patterns used by community clients and helps keep requests working without manual cookie/crumb management.

## Error Behavior

- Non‑2xx HTTP codes: returned as errors with status and limited response body (1 MiB read cap in code path).
- Envelope errors: the top‑level `quoteSummary.error` field is checked; if present, it’s returned as an error.
- Empty results: if `quoteSummary.result` is empty, an error is returned.

## Rate Limiting & Reliability

Yahoo Finance’s endpoints are unofficial and subject to throttling and anti‑abuse protections. Recommendations:

- Concurrency limits: cap concurrent requests (e.g., 5–10) to avoid bursts.
- Backoff on 429/503: exponential backoff with jitter (e.g., 250ms → 2s) and a small retry budget (e.g., 2–3 tries).
- Steady QPS: keep sustained rate modest (e.g., < 5 QPS per IP), especially for large batches.
- Caching: cache recent responses for short durations when feasible to reduce repeated hits.
- Rotate modules: request only the modules you need to minimize payload and server work.
- Persist cookies: if you build a long‑running tool, consider persisting cookies between sessions to reduce crumb fetches (this project currently uses in‑memory jar only).

The current client implements a single automatic retry upon detecting crumb invalidation. If you plan heavy usage, consider adding a generic backoff/retry wrapper around `API` calls in your application layer.

## Typed View

- `QuoteSummaryTyped`: a convenient struct mapping the most commonly used modules (`price`, `summaryDetail`, `financialData`) and optionally `assetProfile` into Go types for table output and easier field access.

## Example (HTTP)

1) Get crumb (must include cookies from warm‑up; here simplified):

```bash
curl -s 'https://query1.finance.yahoo.com/v1/test/getcrumb' \
  -H 'User-Agent: Mozilla/5.0' \
  -c cookies.txt
```

2) Call quoteSummary with modules and crumb:

```bash
curl -s 'https://query1.finance.yahoo.com/v10/finance/quoteSummary/AAPL?modules=price,summaryDetail,financialData&crumb=CRUMB_HERE' \
  -H 'User-Agent: Mozilla/5.0' \
  -b cookies.txt
```

In this project, these steps are handled automatically by `pkg/yfapi.Client`.

## Library Entry Points

- Interface: `pkg/yfapi.API`
  - `QuoteSummary(ctx, symbol, modules) (any, error)`
  - `QuoteSummaryTyped(ctx, symbol, modules) (QuoteSummaryTyped, error)`
- Default client: `pkg/yfapi.DefaultAPI`
- Convenience helpers:
  - `FetchQuoteSummary(ctx, symbol, modules)`
  - `FetchQuoteSummaryTyped(ctx, symbol, modules)`

## CLI Examples

- JSON (pretty): `./yf qs AAPL -o json --pretty`
- Table: `./yf qs AAPL -o table`
- List supported modules: `./yf qs --list-modules`

## Security & Terms

This client uses undocumented endpoints intended for Yahoo’s own web apps. Usage may be rate‑limited or blocked. Do not include secrets, and avoid abusive traffic. Respect Yahoo’s terms in your environment.

