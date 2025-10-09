// Yahoo Finance v7 quote operations.
// Endpoint: https://query1.finance.yahoo.com/v7/finance/quote
// Supports fetching multiple symbols via the `symbols` query parameter.
package yfapi

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
)

// quoteV7Envelope mirrors the v7/finance/quote response shape.
type quoteV7Envelope struct {
    QuoteResponse struct {
        Result []Quote `json:"result"`
        Error  any     `json:"error"`
    } `json:"quoteResponse"`
}

// Quote implements API.Quote using the v7/finance/quote endpoint.
// It supports multiple symbols via a comma-separated query parameter.
func (c *Client) Quote(ctx context.Context, symbols []string) ([]Quote, error) {
    if len(symbols) == 0 {
        return nil, fmt.Errorf("no symbols provided")
    }
    // Build URL
    base := "https://query1.finance.yahoo.com/v7/finance/quote"
    q := url.Values{}
    // Join as provided; Yahoo accepts comma-separated symbols
    q.Set("symbols", strings.Join(symbols, ","))
    u := base + "?" + q.Encode()

    resp, err := c.do(ctx, http.MethodGet, u)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("yahoo finance error: %s: %s", resp.Status, strings.TrimSpace(string(b)))
    }
    var env quoteV7Envelope
    if err := jsonUnmarshal(b, &env); err != nil {
        return nil, err
    }
    // Some responses include error at envelope; return as generic failure
    if env.QuoteResponse.Error != nil {
        return nil, fmt.Errorf("quote error: %v", env.QuoteResponse.Error)
    }
    return env.QuoteResponse.Result, nil
}

