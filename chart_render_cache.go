package yfgo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (c *Client) chartRequestOptions(ctx context.Context, opts ChartOptions) requestOptions {
	ro := requestOptionsFromContext(ctx)
	if ro.cacheTTL == nil {
		ttl := c.defaultCacheTTL
		if !c.defaultCacheTTLSet && ttl > 0 {
			switch buildChartQuery(opts).Get("interval") {
			case "1d", "5d", "1wk", "1mo", "3mo":
				ttl = 4 * time.Hour
			default:
				ttl = time.Minute
			}
		}
		ro.cacheTTL = &ttl
	}
	return ro
}

// ChartPNG fetches chart data and renders a PNG, returning its absolute path.
// With a file cache, PNGs are reused under <cache-dir>/render. An empty Path or
// "auto" selects that path; without caching it selects a fresh temporary file.
// Request cache options apply to both the chart data and rendered image.
func (c *Client) ChartPNG(ctx context.Context, symbol string, chartOpts ChartOptions, renderOpts RenderOptions) (string, error) {
	renderOpts, err := renderOpts.dimensions()
	if err != nil {
		return "", err
	}
	symbol = NormalizeSymbol(symbol)
	if symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}
	chartOpts.ReturnType = "object"
	key := cacheKeyChart(symbol, chartOpts)
	ro := c.chartRequestOptions(ctx, chartOpts)
	ctx = WithCacheOptions(ctx, CacheTTL(*ro.cacheTTL))
	var cachePath string
	if store, ok := c.cache.(*fileCacheStore); ok && !ro.bypassCache && *ro.cacheTTL > 0 {
		renderKey := fmt.Sprintf("%s:%d:%d:%s", key, renderOpts.Width, renderOpts.Height, ChartRendererVersion)
		sum := sha256.Sum256([]byte(renderKey))
		cachePath = filepath.Join(store.root, "render", fmt.Sprintf("%x.png", sum))
	}
	output := renderOpts.Path
	if output == "" || output == "auto" {
		output = cachePath
	}
	if output != "" {
		output, err = filepath.Abs(output)
		if err != nil {
			return "", err
		}
	}
	if cachePath != "" {
		cachePath, err = filepath.Abs(cachePath)
		if err != nil {
			return "", err
		}
		if !ro.forceRefresh {
			if entry, ok, err := c.cache.Get(ctx, key); err == nil && ok && chartPNGCacheHit(cachePath, entry, ro, renderOpts) {
				if output != cachePath {
					if err := copyChartPNG(cachePath, output); err != nil {
						return "", err
					}
				}
				return output, nil
			}
		}
	}
	result, err := c.ChartTyped(ctx, symbol, chartOpts)
	if err != nil {
		return "", err
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
		tmp, err := os.CreateTemp("", "yf-chart-*.png")
		if err != nil {
			return "", err
		}
		output = tmp.Name()
		if err := tmp.Close(); err != nil {
			os.Remove(output)
			return "", err
		}
		// RenderChartPNG will create the file atomically.
		os.Remove(output)
	}
	target := output
	if cachePath != "" {
		target = cachePath
	}
	renderOpts.Path = target
	if err := RenderChartPNG(result, renderOpts); err != nil {
		return "", err
	}
	if cachePath != "" {
		// Age the image from the source data's fetch time, so rendering a new
		// size does not extend the lifetime of old chart data.
		if entry, ok, err := c.cache.Get(ctx, key); err == nil && ok {
			if err := os.Chtimes(cachePath, entry.StoredAt, entry.StoredAt); err != nil {
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

func chartPNGCacheHit(path string, entry CacheEntry, ro requestOptions, opts RenderOptions) bool {
	ttl := entry.TTL
	if ro.cacheTTL != nil && (ttl == 0 || *ro.cacheTTL < ttl) {
		ttl = *ro.cacheTTL
	}
	if ttl <= 0 || time.Since(entry.StoredAt) > ttl {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.ModTime().Before(entry.StoredAt) {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	config, err := png.DecodeConfig(f)
	return err == nil && config.Width == opts.Width && config.Height == opts.Height
}

func copyChartPNG(source, output string) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeChartFile(output, func(w io.Writer) error {
		_, err := io.Copy(w, f)
		return err
	})
}
