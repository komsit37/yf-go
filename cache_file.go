package yfgo

import (
	"context"
	"crypto/sha1" // #nosec G505 -- SHA1 is fine for file naming
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewFileCacheStore creates a cache backed by files within dir. The directory
// is created with 0o700 permissions when missing.
func NewFileCacheStore(dir string) (CacheStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("cache directory cannot be empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &fileCacheStore{root: dir}, nil
}

type fileCacheStore struct {
	root string
}

type fileCacheRecord struct {
	Payload  []byte        `json:"payload"`
	StoredAt time.Time     `json:"storedAt"`
	TTL      time.Duration `json:"ttl"`
}

func (f *fileCacheStore) Get(_ context.Context, key string) (CacheEntry, bool, error) {
	path := f.pathForKey(key)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CacheEntry{}, false, nil
	}
	if err != nil {
		return CacheEntry{}, false, err
	}
	var rec fileCacheRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		_ = os.Remove(path)
		return CacheEntry{}, false, fmt.Errorf("invalid cache record: %w", err)
	}
	if rec.TTL > 0 && time.Since(rec.StoredAt) > rec.TTL {
		_ = os.Remove(path)
		return CacheEntry{}, false, nil
	}
	return CacheEntry{
		Payload:  rec.Payload,
		StoredAt: rec.StoredAt,
		TTL:      rec.TTL,
	}, true, nil
}

func (f *fileCacheStore) Set(_ context.Context, key string, entry CacheEntry) error {
	if entry.Payload == nil {
		return f.Delete(context.Background(), key)
	}
	rec := fileCacheRecord{
		Payload:  entry.Payload,
		StoredAt: entry.StoredAt.UTC(),
		TTL:      entry.TTL,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmpPath := f.pathForKey(key) + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, f.pathForKey(key))
}

func (f *fileCacheStore) Delete(_ context.Context, key string) error {
	err := os.Remove(f.pathForKey(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (f *fileCacheStore) pathForKey(key string) string {
	sum := sha1.Sum([]byte(key)) // #nosec G401 -- collisions and speed acceptable
	name := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(f.root, name)
}
