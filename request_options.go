package yfgo

import (
	"context"
	"time"
)

type cacheOptionKey struct{}

// RequestOption configures cache behaviour for a single API call via context.
type RequestOption func(*requestOptions)

type requestOptions struct {
	cacheTTL     *time.Duration
	forceRefresh bool
	bypassCache  bool
}

// WithCacheOptions returns a new context that carries request-scoped cache options.
func WithCacheOptions(ctx context.Context, opts ...RequestOption) context.Context {
	current := requestOptionsFromContext(ctx)
	for _, opt := range opts {
		opt(&current)
	}
	return context.WithValue(ctx, cacheOptionKey{}, current)
}

// CacheTTL overrides the cache TTL for this request. Values <=0 disable caching.
func CacheTTL(ttl time.Duration) RequestOption {
	return func(ro *requestOptions) {
		// copy value to ensure pointer remains stable
		v := ttl
		ro.cacheTTL = &v
	}
}

// ForceRefresh skips cache reads and refreshes the stored value after a successful fetch.
func ForceRefresh() RequestOption {
	return func(ro *requestOptions) {
		ro.forceRefresh = true
	}
}

// BypassCache skips cache reads and writes for this request.
func BypassCache() RequestOption {
	return func(ro *requestOptions) {
		ro.bypassCache = true
	}
}

func requestOptionsFromContext(ctx context.Context) requestOptions {
	v := ctx.Value(cacheOptionKey{})
	if v == nil {
		return requestOptions{}
	}
	if opts, ok := v.(requestOptions); ok {
		return opts
	}
	return requestOptions{}
}
