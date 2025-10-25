// Yahoo Finance v10 quoteSummary operations.
// Endpoint: https://query1.finance.yahoo.com/v10/finance/quoteSummary/{symbol}
// Supports modules via the `modules` query parameter and requires a valid crumb.
package yfgo

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
	c.saveState()
	return nil
}

// QuoteSummary calls the Yahoo Finance quoteSummary endpoint.
func (c *Client) QuoteSummary(ctx context.Context, symbol string, modules []QuoteSummaryModule) (any, error) {
	reqOpts := requestOptionsFromContext(ctx)
	if len(modules) == 0 {
		fetched, err := c.fetchQuoteSummary(ctx, symbol, nil)
		if err != nil {
			return nil, err
		}
		c.storeQuoteSummaryModules(ctx, symbol, fetched, reqOpts, nil)
		return fetched, nil
	}

	requested := dedupeQuoteSummaryModules(modules)
	moduleData := make(map[string]any, len(requested))
	var missing []QuoteSummaryModule

	if !reqOpts.forceRefresh {
		for _, mod := range requested {
			if value, ok := c.loadQuoteSummaryModule(ctx, symbol, mod, reqOpts); ok {
				moduleData[mod.String()] = value
				continue
			}
			missing = append(missing, mod)
		}
	} else {
		missing = append(missing, requested...)
	}

	var fetched map[string]any
	if len(missing) > 0 {
		var err error
		fetched, err = c.fetchQuoteSummary(ctx, symbol, missing)
		if err != nil {
			return nil, err
		}
		for name, value := range fetched {
			moduleData[name] = value
		}
		c.storeQuoteSummaryModules(ctx, symbol, fetched, reqOpts, nil)
	}

	result := make(map[string]any, len(moduleData))
	for _, mod := range requested {
		name := mod.String()
		if val, ok := moduleData[name]; ok {
			result[name] = val
		}
	}
	if fetched != nil {
		for name, value := range fetched {
			if _, exists := result[name]; !exists {
				result[name] = value
			}
		}
	}

	return result, nil
}

func (c *Client) fetchQuoteSummary(ctx context.Context, symbol string, modules []QuoteSummaryModule) (map[string]any, error) {
	if err := c.ensureCrumb(ctx); err != nil {
		return nil, err
	}
	base := fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s", url.PathEscape(symbol))
	q := url.Values{}
	if len(modules) > 0 {
		q.Set("modules", strings.Join(ModulesToStrings(modules), ","))
	}
	q.Set("crumb", c.crumb)
	u := base + "?" + q.Encode()

	result, status, body, err := c.callQuoteSummary(ctx, u)
	if err == nil {
		return result, nil
	}
	if status == http.StatusUnauthorized || strings.Contains(strings.ToLower(body), "invalid crumb") {
		c.crumb = ""
		if err2 := c.ensureCrumb(ctx); err2 != nil {
			return nil, err
		}
		q.Set("crumb", c.crumb)
		u = base + "?" + q.Encode()
		result, _, _, err = c.callQuoteSummary(ctx, u)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, err
}

func (c *Client) callQuoteSummary(ctx context.Context, u string) (map[string]any, int, string, error) {
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
	if root, ok := env.QuoteSummary.Result[0].(map[string]any); ok {
		return root, resp.StatusCode, string(b), nil
	}
	// Fall back to marshaling to a generic map if the decoder gave a different type.
	raw, err := json.Marshal(env.QuoteSummary.Result[0])
	if err != nil {
		return nil, resp.StatusCode, string(b), fmt.Errorf("unexpected quoteSummary payload: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, resp.StatusCode, string(b), fmt.Errorf("unexpected quoteSummary payload: %w", err)
	}
	return out, resp.StatusCode, string(b), nil
}

func (c *Client) loadQuoteSummaryModule(ctx context.Context, symbol string, module QuoteSummaryModule, opts requestOptions) (any, bool) {
	if ttl, ok := c.moduleCacheTTL[module]; ok && ttl <= 0 {
		c.cacheDelete(ctx, cacheKeyQuoteSummaryModule(symbol, module))
		return nil, false
	}
	key := cacheKeyQuoteSummaryModule(symbol, module)
	if payload, ok := c.cacheGet(ctx, key, opts); ok {
		var cached any
		if err := jsonUnmarshal(payload, &cached); err == nil {
			return cached, true
		}
		c.cacheDelete(ctx, key)
	}
	return nil, false
}

func (c *Client) storeQuoteSummaryModules(ctx context.Context, symbol string, fetched map[string]any, opts requestOptions, allow map[string]struct{}) {
	if len(fetched) == 0 {
		return
	}
	for name, value := range fetched {
		if len(allow) > 0 {
			if _, ok := allow[name]; !ok {
				continue
			}
		}
		mod, ok := ParseQuoteSummaryModule(name)
		if !ok {
			continue
		}
		c.storeQuoteSummaryModule(ctx, symbol, mod, value, opts)
	}
}

func (c *Client) storeQuoteSummaryModule(ctx context.Context, symbol string, module QuoteSummaryModule, value any, opts requestOptions) {
	key := cacheKeyQuoteSummaryModule(symbol, module)
	if ttl, ok := c.moduleCacheTTL[module]; ok {
		if ttl <= 0 {
			c.cacheDelete(ctx, key)
			return
		}
		ttlCopy := ttl
		c.cacheStoreValueWithTTL(ctx, key, opts, value, &ttlCopy)
		return
	}
	c.cacheStoreValue(ctx, key, opts, value)
}

func dedupeQuoteSummaryModules(mods []QuoteSummaryModule) []QuoteSummaryModule {
	if len(mods) <= 1 {
		return append([]QuoteSummaryModule(nil), mods...)
	}
	seen := make(map[QuoteSummaryModule]struct{}, len(mods))
	out := make([]QuoteSummaryModule, 0, len(mods))
	for _, m := range mods {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
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
