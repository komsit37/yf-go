// Yahoo Finance v10 quoteSummary operations.
// Endpoint: https://query1.finance.yahoo.com/v10/finance/quoteSummary/{symbol}
// Supports modules via the `modules` query parameter and requires a valid crumb.
package yfapi

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
)

// quoteSummaryEnvelope mirrors the top-level structure returned by the
// Yahoo Finance quoteSummary endpoint.
type quoteSummaryEnvelope struct {
    QuoteSummary struct {
        Result []any `json:"result"`
        Error  any   `json:"error"`
    } `json:"quoteSummary"`
}

// ensureCrumb fetches a valid cookie+crumb pair required by the v10 API.
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
func (c *Client) QuoteSummary(ctx context.Context, symbol string, modules []QuoteSummaryModule) (any, error) {
    if err := c.ensureCrumb(ctx); err != nil {
        return nil, err
    }
    // Build URL with crumb
    base := fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s", url.PathEscape(symbol))
    q := url.Values{}
    if len(modules) > 0 {
        q.Set("modules", strings.Join(ModulesToStrings(modules), ","))
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

// QuoteSummaryTyped implements API.QuoteSummaryTyped using this client instance.
func (c *Client) QuoteSummaryTyped(ctx context.Context, symbol string, modules []QuoteSummaryModule) (QuoteSummaryTyped, error) {
    raw, err := c.QuoteSummary(ctx, symbol, modules)
    if err != nil {
        return QuoteSummaryTyped{}, err
    }
    var out QuoteSummaryTyped
    b, _ := json.Marshal(raw)
    _ = json.Unmarshal(b, &out)
    return out, nil
}

