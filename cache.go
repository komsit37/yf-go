package yfgo

import (
	"context"
	"sync"
	"time"
)

// CacheStore defines a minimal interface for caching raw Yahoo Finance payloads.
// Implementations must be safe for concurrent use.
type CacheStore interface {
	Get(ctx context.Context, key string) (CacheEntry, bool, error)
	Set(ctx context.Context, key string, entry CacheEntry) error
	Delete(ctx context.Context, key string) error
}

// CacheEntry represents a cached payload along with metadata to evaluate expiry.
type CacheEntry struct {
	Payload  []byte
	StoredAt time.Time
	TTL      time.Duration
}

// memoryCacheStore provides an in-memory, goroutine-safe cache implementation.
type memoryCacheStore struct {
	mu    sync.RWMutex
	items map[string]CacheEntry
}

// NewMemoryCacheStore returns a concurrent-safe in-memory cache store.
func NewMemoryCacheStore() CacheStore {
	return &memoryCacheStore{
		items: make(map[string]CacheEntry),
	}
}

func (m *memoryCacheStore) Get(_ context.Context, key string) (CacheEntry, bool, error) {
	m.mu.RLock()
	entry, ok := m.items[key]
	m.mu.RUnlock()
	return entry, ok, nil
}

func (m *memoryCacheStore) Set(_ context.Context, key string, entry CacheEntry) error {
	m.mu.Lock()
	if entry.Payload == nil {
		delete(m.items, key)
	} else {
		m.items[key] = entry
	}
	m.mu.Unlock()
	return nil
}

func (m *memoryCacheStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.items, key)
	m.mu.Unlock()
	return nil
}
