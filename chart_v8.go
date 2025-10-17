package yfgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ChartOptions configures the v8/finance/chart request.
// Zero values trigger Yahoo's defaults, but we set sensible defaults when nil.
type ChartOptions struct {
	Interval        string
	Range           string
	Period1         *int64
	Period2         *int64
	IncludePrePost  *bool
	Events          string
	Lang            string
	UseYfid         *bool
	ReturnType      string
	AdditionalQuery url.Values
}

type chartEnvelope struct {
	Chart struct {
		Result []any `json:"result"`
		Error  *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// Chart implements API.Chart using the v8/finance/chart endpoint.
func (c *Client) Chart(ctx context.Context, symbol string, opts ChartOptions) (any, error) {
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	reqOpts := requestOptionsFromContext(ctx)
	key := cacheKeyChart(symbol, opts)
	if !reqOpts.forceRefresh {
		if payload, ok := c.cacheGet(ctx, key, reqOpts); ok {
			var cached any
			if err := jsonUnmarshal(payload, &cached); err == nil {
				return cached, nil
			}
			c.cacheDelete(ctx, key)
		}
	}
	c.ensureSession(ctx)

	base := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s", url.PathEscape(symbol))
	q := buildChartQuery(opts)

	// Attach crumb best-effort; many responses work without it but this improves reliability.
	if c.crumb == "" {
		_ = c.ensureCrumb(ctx)
	}
	if c.crumb != "" {
		q.Set("crumb", c.crumb)
	}

	u := base + "?" + q.Encode()

	resp, err := c.do(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// If unauthorized, refresh crumb once and retry.
		if resp.StatusCode == http.StatusUnauthorized {
			c.crumb = ""
			if err := c.ensureCrumb(ctx); err == nil && c.crumb != "" {
				q.Set("crumb", c.crumb)
				u = base + "?" + q.Encode()
				if resp2, err2 := c.do(ctx, http.MethodGet, u); err2 == nil {
					defer resp2.Body.Close()
					body2, _ := io.ReadAll(io.LimitReader(resp2.Body, 1<<20))
					if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
						result, err := decodeChart(body2)
						if err != nil {
							return nil, err
						}
						c.cacheStoreValue(ctx, key, reqOpts, result)
						return result, nil
					}
					return nil, fmt.Errorf("yahoo finance error: %s: %s", resp2.Status, strings.TrimSpace(string(body2)))
				}
			}
		}
		return nil, fmt.Errorf("yahoo finance error: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	result, err := decodeChart(body)
	if err != nil {
		return nil, err
	}
	c.cacheStoreValue(ctx, key, reqOpts, result)
	return result, nil
}

func decodeChart(body []byte) (any, error) {
	var env chartEnvelope
	if err := jsonUnmarshal(body, &env); err != nil {
		return nil, err
	}
	if env.Chart.Error != nil {
		desc := env.Chart.Error.Description
		if desc == "" && env.Chart.Error.Code != "" {
			desc = env.Chart.Error.Code
		}
		return nil, fmt.Errorf("chart error: %s", desc)
	}
	if len(env.Chart.Result) == 0 {
		return nil, fmt.Errorf("no chart data returned")
	}
	return env.Chart.Result[0], nil
}

// ChartTyped implements API.ChartTyped by forcing the "object" return style and
// decoding the response into ChartResult.
func (c *Client) ChartTyped(ctx context.Context, symbol string, opts ChartOptions) (ChartResult, error) {
	// Force return=object for predictable structure while preserving caller intent elsewhere.
	optsTyped := opts
	if optsTyped.ReturnType == "" || optsTyped.ReturnType == "array" {
		optsTyped.ReturnType = "object"
	}
	raw, err := c.Chart(ctx, symbol, optsTyped)
	if err != nil {
		return ChartResult{}, err
	}
	var out ChartResult
	b, _ := json.Marshal(raw)
	if err := json.Unmarshal(b, &out); err != nil {
		return ChartResult{}, err
	}
	return out, nil
}
