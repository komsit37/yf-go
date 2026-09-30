package yfgo

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestChartPNGFileCache(t *testing.T) {
	t.Setenv("YF_HOME", t.TempDir())
	dir := t.TempDir()
	store, err := NewFileCacheStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	newClient := func() *Client {
		client := NewClient(WithCacheStore(store))
		client.sessionWarmed, client.crumb = true, "crumb"
		client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if !strings.HasSuffix(req.URL.Path, "/7203.T") {
				t.Fatalf("unexpected symbol path: %s", req.URL.Path)
			}
			result := chartFixture()
			result.Meta.Symbol = "7203.T"
			data, err := json.Marshal(map[string]any{"chart": map[string]any{"result": []ChartResult{result}, "error": nil}})
			if err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
		})
		return client
	}
	ctx := context.Background()
	query := ChartOptions{Range: "6mo", Interval: "1d"}
	keyOpts := query
	keyOpts.ReturnType = "object"
	key := cacheKeyChart("7203.T", keyOpts)
	client := newClient()
	path, err := client.ChartPNG(ctx, "JP:7203", query, RenderOptions{Path: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || filepath.Dir(path) != filepath.Join(dir, "render") || calls != 1 {
		t.Fatalf("path=%s requests=%d", path, calls)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// A new client represents a new CLI process. The atomic writer would
	// replace the inode on a rerender, so SameFile verifies image reuse too.
	client = newClient()
	second, err := client.ChartPNG(ctx, "7203.T", query, RenderOptions{Path: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil || path != second || !os.SameFile(info, secondInfo) || calls != 1 {
		t.Fatalf("cache miss: path=%s requests=%d error=%v", second, calls, err)
	}
	entry, ok, err := store.Get(ctx, key)
	if err != nil || !ok || entry.TTL != 4*time.Hour {
		t.Fatalf("daily data TTL = %v, ok=%v error=%v", entry.TTL, ok, err)
	}
	// A second size reuses chart data and has a separate render key.
	resized, err := client.ChartPNG(ctx, "jp:7203", query, RenderOptions{Path: "auto", Width: 640, Height: 320})
	if err != nil || resized == path || calls != 1 {
		t.Fatalf("resize: path=%s requests=%d error=%v", resized, calls, err)
	}
	explicit := filepath.Join(t.TempDir(), "output.png")
	copyPath, err := client.ChartPNG(ctx, "JP:7203", query, RenderOptions{Path: explicit})
	if err != nil || copyPath != explicit || calls != 1 {
		t.Fatalf("explicit output: path=%s requests=%d error=%v", copyPath, calls, err)
	}
	if got, err := os.ReadFile(explicit); err != nil {
		t.Fatal(err)
	} else if want, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
		t.Fatal("explicit output differs from cached PNG")
	}
	refresh := WithCacheOptions(ctx, ForceRefresh())
	if _, err := client.ChartPNG(refresh, "JP:7203", query, RenderOptions{Path: "auto"}); err != nil || calls != 2 {
		t.Fatalf("force refresh: requests=%d error=%v", calls, err)
	}
	// Refreshing one size invalidates other images of the old source data.
	oldResize, err := os.Stat(resized)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ChartPNG(ctx, "JP:7203", query, RenderOptions{Path: "auto", Width: 640, Height: 320}); err != nil || calls != 2 {
		t.Fatalf("resize after refresh: requests=%d error=%v", calls, err)
	}
	newResize, err := os.Stat(resized)
	if err != nil || os.SameFile(oldResize, newResize) {
		t.Fatal("old resized image survived a source refresh")
	}
	before, ok, err := store.Get(ctx, key)
	if err != nil || !ok {
		t.Fatal("missing source data")
	}
	for _, option := range []RequestOption{BypassCache(), CacheTTL(0)} {
		uncached, err := client.ChartPNG(WithCacheOptions(ctx, option), "JP:7203", query, RenderOptions{Path: "auto"})
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(uncached)
		if uncached == path || filepath.Dir(uncached) == filepath.Join(dir, "render") {
			t.Fatalf("bypass reused cache path: %s", uncached)
		}
	}
	if calls != 4 {
		t.Fatalf("bypass must fetch each time: requests=%d", calls)
	}
	after, ok, err := store.Get(ctx, key)
	if err != nil || !ok || !after.StoredAt.Equal(before.StoredAt) {
		t.Fatal("bypass updated source cache")
	}
	// Simulate a source entry older than a shorter user TTL without sleeping.
	after.StoredAt = time.Now().Add(-2 * time.Minute)
	if err := store.Set(ctx, key, after); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ChartPNG(WithCacheOptions(ctx, CacheTTL(time.Minute)), "JP:7203", query, RenderOptions{Path: "auto"}); err != nil || calls != 5 {
		t.Fatalf("TTL override: requests=%d error=%v", calls, err)
	}
	entry, ok, err = store.Get(ctx, key)
	if err != nil || !ok || entry.TTL != time.Minute {
		t.Fatalf("override not stored: TTL=%v ok=%v error=%v", entry.TTL, ok, err)
	}
}

func TestChartDataIntervalTTL(t *testing.T) {
	t.Setenv("YF_HOME", t.TempDir())
	for _, tt := range []struct {
		interval string
		options  []ClientOption
		request  []RequestOption
		want     time.Duration
	}{
		{"", nil, nil, 4 * time.Hour},
		{"1d", nil, nil, 4 * time.Hour}, {"5d", nil, nil, 4 * time.Hour},
		{"1wk", nil, nil, 4 * time.Hour}, {"1mo", nil, nil, 4 * time.Hour}, {"3mo", nil, nil, 4 * time.Hour},
		{"1m", nil, nil, time.Minute}, {"1h", nil, nil, time.Minute}, {"60m", nil, nil, time.Minute},
		{"1d", []ClientOption{WithDefaultCacheTTL(6 * time.Hour)}, nil, 6 * time.Hour},
		{"1d", nil, []RequestOption{CacheTTL(8 * time.Hour)}, 8 * time.Hour},
		{"1m", nil, []RequestOption{CacheTTL(10 * time.Second)}, 10 * time.Second},
		{"1d", nil, []RequestOption{CacheTTL(0)}, 0},
		{"1d", []ClientOption{WithCacheDisabled()}, nil, 0},
	} {
		t.Run(tt.interval+"/"+tt.want.String(), func(t *testing.T) {
			store := NewMemoryCacheStore()
			options := append([]ClientOption{WithCacheStore(store)}, tt.options...)
			client := NewClient(options...)
			client.sessionWarmed, client.crumb = true, "crumb"
			client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"chart":{"result":[{"meta":{}}],"error":null}}`)), Header: make(http.Header)}, nil
			})
			ctx := WithCacheOptions(context.Background(), tt.request...)
			opts := ChartOptions{Interval: tt.interval}
			if _, err := client.Chart(ctx, "AAPL", opts); err != nil {
				t.Fatal(err)
			}
			entry, ok, err := store.Get(ctx, cacheKeyChart("AAPL", opts))
			if err != nil || ok != (tt.want > 0) || entry.TTL != tt.want {
				t.Fatalf("TTL=%v ok=%v error=%v, want TTL=%v", entry.TTL, ok, err, tt.want)
			}
		})
	}
}
