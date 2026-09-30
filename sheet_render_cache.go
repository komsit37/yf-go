package yfgo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// SheetPNG fetches a chart and fundamentals using their existing cache entries,
// then returns an absolute PNG path. Cached sheets live in <cache-dir>/render.
// A hit requires every source to be fresh; refreshing any source invalidates it.
func (c *Client) SheetPNG(ctx context.Context, symbol string, chartOpts ChartOptions, opts SheetRenderOptions) (string, error) {
	opts, err := opts.dimensions()
	if err != nil {
		return "", err
	}
	symbol = NormalizeSymbol(symbol)
	if symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}
	chartOpts.ReturnType = "object"
	ro := requestOptionsFromContext(ctx)
	chartRO := c.chartRequestOptions(ctx, chartOpts)
	var cachePath string
	canCache := !ro.bypassCache && *chartRO.cacheTTL > 0 && (ro.cacheTTL == nil || *ro.cacheTTL > 0)
	for _, module := range sheetModules {
		if ttl, ok := c.moduleCacheTTL[module]; ok && ttl <= 0 {
			canCache = false
		}
	}
	if store, ok := c.cache.(*fileCacheStore); ok && canCache {
		keyOpts := opts
		keyOpts.Path = ""
		encoded, _ := json.Marshal(keyOpts)
		sum := sha256.Sum256([]byte(cacheKeyChart(symbol, chartOpts) + ":" + SheetRendererVersion + ":" + ChartRendererVersion + ":" + string(encoded)))
		cachePath, err = filepath.Abs(filepath.Join(store.root, "render", fmt.Sprintf("sheet-%x.png", sum)))
		if err != nil {
			return "", err
		}
	}
	output := opts.Path
	if output == "" || output == "auto" {
		output = cachePath
	}
	if output != "" {
		output, err = filepath.Abs(output)
		if err != nil {
			return "", err
		}
	}
	if cachePath != "" && !ro.forceRefresh {
		if newest, ok := c.sheetSources(ctx, symbol, chartOpts); ok && sheetPNGCacheHit(cachePath, newest, opts) {
			if output != cachePath {
				if err := copyChartPNG(cachePath, output); err != nil {
					return "", err
				}
			}
			return output, nil
		}
	}
	result, err := c.ChartTyped(ctx, symbol, chartOpts)
	if err != nil {
		return "", err
	}
	summary, err := c.QuoteSummary(ctx, symbol, sheetModules)
	if err != nil {
		return "", err
	}
	data, err := ParseSheetData(summary)
	if err != nil {
		return "", err
	}
	// Yahoo omits unsupported modules for some securities. Cache their absence
	// as null, avoiding repeated requests for the same successfully fetched sheet.
	if root, ok := summary.(map[string]any); ok {
		for _, module := range sheetModules {
			if _, exists := root[module.String()]; !exists {
				c.storeQuoteSummaryModule(ctx, symbol, module, nil, ro)
			}
		}
	}
	if result.Meta.Symbol == "" {
		result.Meta.Symbol = symbol
	}
	if result.Meta.Range == "" {
		result.Meta.Range = buildChartQuery(chartOpts).Get("range")
		if result.Meta.Range == "" {
			result.Meta.Range = "custom"
		}
	}
	if output == "" {
		temporary, err := os.CreateTemp("", "yf-sheet-*.png")
		if err != nil {
			return "", err
		}
		output = temporary.Name()
		if err := temporary.Close(); err != nil {
			os.Remove(output)
			return "", err
		}
		os.Remove(output)
	}
	target := output
	if cachePath != "" {
		target = cachePath
	}
	opts.Path = target
	if err := RenderSheetPNG(result, data, opts); err != nil {
		return "", err
	}
	if cachePath != "" {
		if newest, ok := c.sheetSources(ctx, symbol, chartOpts); ok {
			if err := os.Chtimes(cachePath, newest, newest); err != nil {
				return "", err
			}
		}
		if output != cachePath {
			if err := copyChartPNG(cachePath, output); err != nil {
				return "", err
			}
		}
	}
	return output, nil
}

func (c *Client) sheetSources(ctx context.Context, symbol string, opts ChartOptions) (time.Time, bool) {
	if c.cache == nil {
		return time.Time{}, false
	}
	ro := requestOptionsFromContext(ctx)
	chartRO := c.chartRequestOptions(ctx, opts)
	newest := time.Time{}
	check := func(key string, ro requestOptions) bool {
		if _, ok := c.cacheGet(ctx, key, ro); !ok {
			return false
		}
		entry, ok, err := c.cache.Get(ctx, key)
		if err != nil || !ok {
			return false
		}
		if entry.StoredAt.After(newest) {
			newest = entry.StoredAt
		}
		return true
	}
	if !check(cacheKeyChart(symbol, opts), chartRO) {
		return newest, false
	}
	for _, module := range sheetModules {
		if !check(cacheKeyQuoteSummaryModule(symbol, module), ro) {
			return newest, false
		}
	}
	return newest, true
}

func sheetPNGCacheHit(path string, newest time.Time, opts SheetRenderOptions) bool {
	info, err := os.Stat(path)
	if err != nil || info.ModTime().Before(newest) {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	config, err := png.DecodeConfig(f)
	return err == nil && config.Width == opts.Width && config.Height >= opts.Height-46 && config.Height <= 8192
}
