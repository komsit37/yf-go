package yfgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	c.ensureSession(ctx)

	base := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s", url.PathEscape(symbol))
	q := url.Values{}
	for k, vals := range opts.AdditionalQuery {
		for _, v := range vals {
			q.Add(k, v)
		}
	}

	if opts.Interval != "" {
		q.Set("interval", opts.Interval)
	}
	if opts.Range != "" {
		q.Set("range", opts.Range)
	}
	if opts.Period1 != nil {
		q.Set("period1", strconv.FormatInt(*opts.Period1, 10))
	}
	if opts.Period2 != nil {
		q.Set("period2", strconv.FormatInt(*opts.Period2, 10))
	}
	if opts.IncludePrePost != nil {
		q.Set("includePrePost", strconv.FormatBool(*opts.IncludePrePost))
	}
	if opts.Events != "" {
		q.Set("events", opts.Events)
	}
	if opts.Lang != "" {
		q.Set("lang", opts.Lang)
	}
	if opts.UseYfid != nil {
		q.Set("useYfid", strconv.FormatBool(*opts.UseYfid))
	}
	if opts.ReturnType != "" {
		q.Set("return", opts.ReturnType)
	}

	// Apply defaults when options omitted.
	if _, ok := q["interval"]; !ok {
		q.Set("interval", "1d")
	}
	if _, ok := q["events"]; !ok {
		q.Set("events", "div|split|earn")
	}
	if _, ok := q["lang"]; !ok {
		q.Set("lang", "en-US")
	}
	if _, ok := q["return"]; !ok {
		q.Set("return", "array")
	}
	if _, ok := q["includePrePost"]; !ok {
		q.Set("includePrePost", "true")
	}
	if _, ok := q["useYfid"]; !ok {
		q.Set("useYfid", "true")
	}
	// Yahoo requires either range or period1.
	if _, hasRange := q["range"]; !hasRange && opts.Period1 == nil {
		q.Set("range", "1mo")
	}

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
						return decodeChart(body2)
					}
					return nil, fmt.Errorf("yahoo finance error: %s: %s", resp2.Status, strings.TrimSpace(string(body2)))
				}
			}
		}
		return nil, fmt.Errorf("yahoo finance error: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return decodeChart(body)
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
