package yfgo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientQuoteCaching(t *testing.T) {
	t.Setenv("YF_HOME", t.TempDir())

	t.Run("cache hit avoids subsequent request", func(t *testing.T) {
		hits := int32(0)
		srv := newQuoteServer(t, &hits)
		client := newTestClient(t, srv, 5*time.Minute)

		ctx := context.Background()
		res1, err := client.Quote(ctx, []string{"AAPL"})
		if err != nil {
			t.Fatalf("Quote() first call error: %v", err)
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Fatalf("expected 1 request, got %d", got)
		}
		if res1[0].Symbol != "AAPL-1" {
			t.Fatalf("unexpected symbol %q", res1[0].Symbol)
		}

		res2, err := client.Quote(context.Background(), []string{"AAPL"})
		if err != nil {
			t.Fatalf("Quote() second call error: %v", err)
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Fatalf("expected no additional requests, got %d", got)
		}
		if res2[0].Symbol != "AAPL-1" {
			t.Fatalf("expected cached symbol AAPL-1, got %s", res2[0].Symbol)
		}
	})

	t.Run("expiry triggers refresh", func(t *testing.T) {
		hits := int32(0)
		srv := newQuoteServer(t, &hits)
		client := newTestClient(t, srv, 20*time.Millisecond)

		if _, err := client.Quote(context.Background(), []string{"AAPL"}); err != nil {
			t.Fatalf("Quote() first call error: %v", err)
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Fatalf("expected 1 request, got %d", got)
		}

		time.Sleep(40 * time.Millisecond)

		res, err := client.Quote(context.Background(), []string{"AAPL"})
		if err != nil {
			t.Fatalf("Quote() after expiry error: %v", err)
		}
		if got := atomic.LoadInt32(&hits); got != 2 {
			t.Fatalf("expected refresh after expiry, got %d requests", got)
		}
		if res[0].Symbol != "AAPL-2" {
			t.Fatalf("expected refreshed symbol AAPL-2, got %s", res[0].Symbol)
		}
	})

	t.Run("force refresh bypasses read but updates cache", func(t *testing.T) {
		hits := int32(0)
		srv := newQuoteServer(t, &hits)
		client := newTestClient(t, srv, 5*time.Minute)

		if _, err := client.Quote(context.Background(), []string{"AAPL"}); err != nil {
			t.Fatalf("Quote() first call error: %v", err)
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Fatalf("expected 1 request, got %d", got)
		}

		ctx := WithCacheOptions(context.Background(), ForceRefresh())
		res, err := client.Quote(ctx, []string{"AAPL"})
		if err != nil {
			t.Fatalf("Quote() force refresh error: %v", err)
		}
		if got := atomic.LoadInt32(&hits); got != 2 {
			t.Fatalf("expected refresh call, got %d requests", got)
		}
		if res[0].Symbol != "AAPL-2" {
			t.Fatalf("expected refreshed symbol AAPL-2, got %s", res[0].Symbol)
		}

		// Subsequent call should reuse refreshed value.
		resCached, err := client.Quote(context.Background(), []string{"AAPL"})
		if err != nil {
			t.Fatalf("Quote() post-refresh error: %v", err)
		}
		if got := atomic.LoadInt32(&hits); got != 2 {
			t.Fatalf("expected cached reuse, got %d requests", got)
		}
		if resCached[0].Symbol != "AAPL-2" {
			t.Fatalf("expected cached refreshed symbol AAPL-2, got %s", resCached[0].Symbol)
		}
	})

	t.Run("bypass skips reads and writes", func(t *testing.T) {
		hits := int32(0)
		srv := newQuoteServer(t, &hits)
		client := newTestClient(t, srv, 5*time.Minute)

		initial, err := client.Quote(context.Background(), []string{"AAPL"})
		if err != nil {
			t.Fatalf("Quote() first call error: %v", err)
		}
		if initial[0].Symbol != "AAPL-1" {
			t.Fatalf("unexpected symbol %s", initial[0].Symbol)
		}

		ctx := WithCacheOptions(context.Background(), BypassCache())
		raw, err := client.Quote(ctx, []string{"AAPL"})
		if err != nil {
			t.Fatalf("Quote() bypass error: %v", err)
		}
		if raw[0].Symbol != "AAPL-2" {
			t.Fatalf("expected uncached symbol AAPL-2, got %s", raw[0].Symbol)
		}
		if got := atomic.LoadInt32(&hits); got != 2 {
			t.Fatalf("expected two requests, got %d", got)
		}

		res, err := client.Quote(context.Background(), []string{"AAPL"})
		if err != nil {
			t.Fatalf("Quote() final call error: %v", err)
		}
		if res[0].Symbol != "AAPL-1" {
			t.Fatalf("expected original cached symbol AAPL-1, got %s", res[0].Symbol)
		}
		if got := atomic.LoadInt32(&hits); got != 2 {
			t.Fatalf("expected no additional requests, got %d", got)
		}
	})
}

func TestFileCacheStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileCacheStore(dir)
	if err != nil {
		t.Fatalf("NewFileCacheStore() error: %v", err)
	}

	entry := CacheEntry{
		Payload:  []byte(`{"symbol":"AAPL-1"}`),
		StoredAt: time.Now().UTC(),
		TTL:      20 * time.Millisecond,
	}
	if err := store.Set(context.Background(), "quote:AAPL", entry); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	got, ok, err := store.Get(context.Background(), "quote:AAPL")
	if err != nil || !ok {
		t.Fatalf("Get() before expiry error=%v ok=%v", err, ok)
	}
	if string(got.Payload) != string(entry.Payload) {
		t.Fatalf("unexpected payload %s", string(got.Payload))
	}

	time.Sleep(30 * time.Millisecond)
	if _, ok, err := store.Get(context.Background(), "quote:AAPL"); err != nil {
		t.Fatalf("Get() after expiry error: %v", err)
	} else if ok {
		t.Fatalf("expected expired entry to be removed")
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected cache directory to be empty after expiry")
	}
}

func TestQuoteSummaryModuleCaching(t *testing.T) {
	hits := int32(0)
	var requested []string
	srv := newQuoteSummaryServer(t, &hits, &requested)
	client := newTestClient(t, srv, 5*time.Minute)

	res1Raw, err := client.QuoteSummary(context.Background(), "AAPL", []QuoteSummaryModule{ModuleAssetProfile, ModulePrice})
	if err != nil {
		t.Fatalf("QuoteSummary() first call error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 request, got %d", got)
	}
	root1, ok := res1Raw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res1Raw)
	}
	if val := moduleValue(root1, "assetProfile"); val != "assetProfile-1" {
		t.Fatalf("unexpected assetProfile value %q", val)
	}
	if val := moduleValue(root1, "price"); val != "price-1" {
		t.Fatalf("unexpected price value %q", val)
	}

	// Price should hit the cache without triggering the upstream server again.
	res2Raw, err := client.QuoteSummary(context.Background(), "AAPL", []QuoteSummaryModule{ModulePrice})
	if err != nil {
		t.Fatalf("QuoteSummary() cached price error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected cache hit without new requests, got %d", got)
	}
	root2, ok := res2Raw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res2Raw)
	}
	if val := moduleValue(root2, "price"); val != "price-1" {
		t.Fatalf("expected cached price-1, got %q", val)
	}

	// Request a cached module plus a new one; only the new module should trigger a request.
	res3Raw, err := client.QuoteSummary(context.Background(), "AAPL", []QuoteSummaryModule{ModuleAssetProfile, ModuleBalanceSheetHistory})
	if err != nil {
		t.Fatalf("QuoteSummary() mixed modules error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected exactly one additional request, got %d", got)
	}
	if len(requested) != 2 {
		t.Fatalf("expected 2 upstream requests recorded, got %d", len(requested))
	}
	if requested[1] != "balanceSheetHistory" {
		t.Fatalf("expected second request for balanceSheetHistory, got %q", requested[1])
	}
	root3, ok := res3Raw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res3Raw)
	}
	if val := moduleValue(root3, "assetProfile"); val != "assetProfile-1" {
		t.Fatalf("expected cached assetProfile-1, got %q", val)
	}
	if val := moduleValue(root3, "balanceSheetHistory"); val != "balanceSheetHistory-2" {
		t.Fatalf("expected fetched balanceSheetHistory-2, got %q", val)
	}
}

func TestQuoteSummaryModuleTTLOverrides(t *testing.T) {
	hits := int32(0)
	var requested []string
	srv := newQuoteSummaryServer(t, &hits, &requested)
	client := newTestClient(
		t,
		srv,
		5*time.Minute,
		WithQuoteSummaryModuleTTLs(map[QuoteSummaryModule]time.Duration{
			ModulePrice:        20 * time.Millisecond,
			ModuleAssetProfile: time.Minute,
		}),
	)

	modules := []QuoteSummaryModule{ModulePrice, ModuleAssetProfile}
	res1Raw, err := client.QuoteSummary(context.Background(), "AAPL", modules)
	if err != nil {
		t.Fatalf("QuoteSummary() first call error: %v", err)
	}
	root1, ok := res1Raw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res1Raw)
	}
	if val := moduleValue(root1, "price"); val != "price-1" {
		t.Fatalf("unexpected price value %q", val)
	}
	if val := moduleValue(root1, "assetProfile"); val != "assetProfile-1" {
		t.Fatalf("unexpected assetProfile value %q", val)
	}

	time.Sleep(30 * time.Millisecond)

	res2Raw, err := client.QuoteSummary(context.Background(), "AAPL", modules)
	if err != nil {
		t.Fatalf("QuoteSummary() second call error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected second request after price TTL expired, got %d", got)
	}
	if len(requested) != 2 {
		t.Fatalf("expected 2 upstream requests recorded, got %d", len(requested))
	}
	if requested[1] != "price" {
		t.Fatalf("expected second request only for price, got %q", requested[1])
	}
	root2, ok := res2Raw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res2Raw)
	}
	if val := moduleValue(root2, "price"); val != "price-2" {
		t.Fatalf("expected refreshed price-2, got %q", val)
	}
	if val := moduleValue(root2, "assetProfile"); val != "assetProfile-1" {
		t.Fatalf("expected cached assetProfile-1, got %q", val)
	}
}

func newQuoteServer(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/v7/finance/quote") {
			http.NotFound(w, r)
			return
		}
		n := atomic.AddInt32(hits, 1)
		resp := map[string]any{
			"quoteResponse": map[string]any{
				"result": []map[string]any{
					{
						"symbol": fmt.Sprintf("AAPL-%d", n),
					},
				},
				"error": nil,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newQuoteSummaryServer(t *testing.T, hits *int32, requested *[]string) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/v10/finance/quoteSummary/") {
			http.NotFound(w, r)
			return
		}
		modulesParam := r.URL.Query().Get("modules")
		if requested != nil {
			*requested = append(*requested, modulesParam)
		}
		n := atomic.AddInt32(hits, 1)
		var mods []string
		if modulesParam != "" {
			for _, item := range strings.Split(modulesParam, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					mods = append(mods, item)
				}
			}
		}
		if len(mods) == 0 {
			mods = []string{"price"}
		}
		payload := make(map[string]any, len(mods))
		for _, m := range mods {
			payload[m] = map[string]any{
				"value": fmt.Sprintf("%s-%d", m, n),
			}
		}
		resp := map[string]any{
			"quoteSummary": map[string]any{
				"result": []map[string]any{payload},
				"error":  nil,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, srv *httptest.Server, ttl time.Duration, extra ...ClientOption) *Client {
	t.Helper()
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	base := srv.Client()
	opts := make([]ClientOption, 0, 1+len(extra))
	opts = append(opts, WithDefaultCacheTTL(ttl))
	opts = append(opts, extra...)
	client := NewClient(opts...)
	client.http = &http.Client{
		Timeout:   5 * time.Second,
		Transport: rewriteRoundTripper{target: target, next: base.Transport},
	}
	client.sessionWarmed = true
	client.crumb = "crumb"
	client.store = nil
	return client
}

func moduleValue(root map[string]any, key string) string {
	mod, ok := root[key].(map[string]any)
	if !ok {
		return ""
	}
	val, _ := mod["value"].(string)
	return val
}

type rewriteRoundTripper struct {
	target *url.URL
	next   http.RoundTripper
}

func (rt rewriteRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = rt.target.Scheme
	clone.URL.Host = rt.target.Host
	clone.Host = rt.target.Host
	return rt.next.RoundTrip(clone)
}
