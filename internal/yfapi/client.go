package yfapi

import (
    "context"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/http/cookiejar"
    "net/url"
    "strings"
    "time"
)

// Client holds HTTP state (cookies/crumb) for Yahoo Finance.
type Client struct {
    http  *http.Client
    crumb string
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

func (c *Client) do(ctx context.Context, method, rawURL string) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
    if err != nil {
        return nil, err
    }
    // Use a realistic browser UA to reduce blocking.
    req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
    req.Header.Set("Accept", "application/json, text/plain, */*")
    req.Header.Set("Accept-Language", "en-US,en;q=0.9")
    return c.http.Do(req)
}

// ensureCrumb fetches a valid cookie+crumb pair.
func (c *Client) ensureCrumb(ctx context.Context) error {
    if c.crumb != "" {
        return nil
    }
    // Step 1: warm cookies by hitting fc.yahoo.com
    if resp, err := c.do(ctx, http.MethodGet, "https://fc.yahoo.com"); err == nil {
        io.Copy(io.Discard, resp.Body)
        resp.Body.Close()
    }

    // Step 2: get crumb
    resp, err := c.do(ctx, http.MethodGet, "https://query1.finance.yahoo.com/v1/test/getcrumb")
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 10240))
        return fmt.Errorf("getcrumb failed: %s: %s", resp.Status, strings.TrimSpace(string(b)))
    }
    b, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
    if err != nil {
        return err
    }
    crumb := strings.TrimSpace(string(b))
    if crumb == "" {
        return errors.New("empty crumb")
    }
    c.crumb = crumb
    return nil
}

// QuoteSummary calls the Yahoo Finance quoteSummary endpoint.
func (c *Client) QuoteSummary(ctx context.Context, symbol string, modules []string) (any, error) {
    if err := c.ensureCrumb(ctx); err != nil {
        return nil, err
    }
    // Build URL with crumb
    base := fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s", url.PathEscape(symbol))
    q := url.Values{}
    if len(modules) > 0 {
        q.Set("modules", strings.Join(modules, ","))
    }
    q.Set("crumb", c.crumb)
    u := base + "?" + q.Encode()

    // First attempt
    result, status, body, err := c.callQuoteSummary(ctx, u)
    if err == nil {
        return result, nil
    }
    // If unauthorized/invalid crumb, refresh once and retry
    if status == http.StatusUnauthorized || strings.Contains(strings.ToLower(body), "invalid crumb") {
        c.crumb = ""
        if err2 := c.ensureCrumb(ctx); err2 != nil {
            return nil, err
        }
        q.Set("crumb", c.crumb)
        u = base + "?" + q.Encode()
        return c.retryQuoteSummary(ctx, u)
    }
    return nil, err
}

func (c *Client) retryQuoteSummary(ctx context.Context, u string) (any, error) {
    result, _, _, err := c.callQuoteSummary(ctx, u)
    return result, err
}

func (c *Client) callQuoteSummary(ctx context.Context, u string) (any, int, string, error) {
    resp, err := c.do(ctx, http.MethodGet, u)
    if err != nil {
        return nil, 0, "", err
    }
    defer resp.Body.Close()
    b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, resp.StatusCode, string(b), fmt.Errorf("yahoo finance error: %s: %s", resp.Status, strings.TrimSpace(string(b)))
    }
    var env quoteSummaryEnvelope
    if err := jsonUnmarshal(b, &env); err != nil {
        return nil, resp.StatusCode, string(b), err
    }
    if env.QuoteSummary.Error != nil {
        return nil, resp.StatusCode, string(b), fmt.Errorf("quoteSummary error: %v", env.QuoteSummary.Error)
    }
    if len(env.QuoteSummary.Result) == 0 {
        return nil, resp.StatusCode, string(b), errors.New("no results returned")
    }
    return env.QuoteSummary.Result[0], resp.StatusCode, string(b), nil
}

// jsonUnmarshal is a tiny wrapper to avoid importing encoding/json in two files.
func jsonUnmarshal(b []byte, v any) error { return defaultJSON.Unmarshal(b, v) }

