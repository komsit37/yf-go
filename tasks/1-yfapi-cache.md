# Plan – Yahoo Finance API caching

## Goals
- Introduce a configurable caching layer that supports both in-memory and file-based stores for Yahoo Finance responses.
- Allow per-client default TTL as well as per-request overrides, including a force-refresh option.
- Ensure behaviour is backed by tests that verify cache hits, misses, expiry, and forced refresh scenarios.

## Tasks
1. **Cache abstractions**
   - Define request cache key generation helper (endpoint + request params).
   - Introduce `CacheStore` interface with `Get`, `Set`, `Delete`.
   - Create `CacheEntry` with payload, stored timestamp, and TTL metadata.
   - Implement basic in-memory cache (`memoryCacheStore`) with RW locking.
2. **File-backed cache**
   - Implement `NewFileCacheStore(dir string)` that persists `CacheEntry` as JSON blobs keyed by hashed filenames.
   - Ensure directory creation (0700) and atomic writes via `*.tmp` files + `os.Rename`.
   - Handle TTL expiry and corruption (delete on invalid read).
3. **Client integration**
   - Add `ClientOptions` and functional option helpers for configuring cache store and default TTL (`WithMemoryCache`, `WithFileCache`, `WithCacheTTL`).
   - Wire cache lookups into `QuoteSummary`, `Quote`, and `Chart` (both typed/untyped paths share helpers).
   - Respect per-request overrides: `WithRequestCacheTTL`, `WithRequestCacheBypass`, `WithForceRefresh`.
   - Ensure force refresh bypasses cache read but still updates cache after successful fetch.
4. **Configuration & CLI plumbing**
   - Surface cache settings via env/flags (`YF_CACHE_TTL`, `YF_CACHE_DIR`, `--cache-ttl`, `--cache-dir`, `--force-refresh`, `--no-cache`).
   - Propagate CLI options as request modifiers when calling `yfgo` methods.
5. **Testing**
   - Add `client_cache_test.go` covering hit/miss, expiry, and force refresh paths using `httptest.Server`.
   - For file cache, use temporary directories and assert files created/updated correctly.
   - Verify CLI integration via test or manual checklist (ensure options plumbed correctly).

## Open Questions
- Decide on cache key format for multi-symbol requests (`Quote`); propose sorted symbols joined by comma.
  - yes
- Confirm whether typed helpers should use same cache entries as the raw endpoints to avoid duplicate storage.
  - yes
- Determine default TTL (e.g., 1 minute?) if not provided; otherwise leave uncached by default.
  - 5 minutes
