package yfgo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Client holds HTTP state (cookies/crumb) for Yahoo Finance.
type Client struct {
	http  *http.Client
	crumb string
	// sessionWarmed indicates we've attempted to prime cookies to reduce 401s.
	sessionWarmed bool
}

// API defines the minimal interface exposed by this package for clients.
// It enables easy dependency injection and testing.
//
// Implementations should be safe for concurrent use.
type API interface {
	// QuoteSummary fetches Yahoo Finance quoteSummary for a symbol with selected modules.
	// It returns the first object of quoteSummary.result as an untyped value.
	QuoteSummary(ctx context.Context, symbol string, modules []QuoteSummaryModule) (any, error)

	// QuoteSummaryTyped is a convenience that maps a subset of modules into a typed struct.
	QuoteSummaryTyped(ctx context.Context, symbol string, modules []QuoteSummaryModule) (QuoteSummaryTyped, error)

	// Quote calls the v7/finance/quote endpoint for one or more symbols and
	// returns the parsed list of quotes (quoteResponse.result).
	Quote(ctx context.Context, symbols []string) ([]Quote, error)
}

// NewClient creates a Yahoo Finance client with cookie jar and timeout.
func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		http: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
	}
}

// Default is a package-level default client.
var Default = NewClient()

// DefaultAPI exposes the default implementation via the API interface.
var DefaultAPI API = Default

// Provide a local json decoder handle for internal use.
var defaultJSON = jsonCodec{}

type jsonCodec struct{}

func (jsonCodec) Unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

func (c *Client) do(ctx context.Context, method, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// Use a realistic browser UA to reduce blocking.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// Setting a finance referer can help Yahoo route requests similarly to browser usage.
	req.Header.Set("Referer", "https://finance.yahoo.com/")
	return c.http.Do(req)
}

// ensureSession performs a lightweight warm-up against Yahoo domains to populate
// cookies that some endpoints expect, helping avoid sporadic 401 responses.
func (c *Client) ensureSession(ctx context.Context) {
	if c.sessionWarmed {
		return
	}
	// Best-effort; ignore errors. Hitting these hosts usually sets required cookies.
	if resp, err := c.do(ctx, http.MethodGet, "https://fc.yahoo.com"); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	if resp, err := c.do(ctx, http.MethodGet, "https://finance.yahoo.com"); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	c.sessionWarmed = true
}

// jsonUnmarshal is a tiny wrapper to avoid importing encoding/json in two files.
func jsonUnmarshal(b []byte, v any) error { return defaultJSON.Unmarshal(b, v) }

// Ensure *Client satisfies API.
var _ API = (*Client)(nil)
