package yfgo

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestClientStatePersistence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("YF_HOME", dir)

	client := NewClient()
	if client.store == nil {
		t.Fatalf("expected store to be initialized")
	}

	client.crumb = "persisted-crumb"

	u, _ := url.Parse("https://query1.finance.yahoo.com/")
	client.http.Jar.SetCookies(u, []*http.Cookie{
		{
			Name:   "B",
			Value:  "abc",
			Domain: ".finance.yahoo.com",
			Path:   "/",
		},
	})
	client.saveState()

	statePath := filepath.Join(dir, "state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}

	client2 := NewClient()
	if got, want := client2.crumb, "persisted-crumb"; got != want {
		t.Fatalf("restored crumb = %q, want %q", got, want)
	}
	cookies := client2.http.Jar.Cookies(u)
	if len(cookies) == 0 {
		t.Fatalf("expected cookies to be restored")
	}
	if !client2.sessionWarmed {
		t.Fatalf("expected sessionWarmed after restoring cookies")
	}

	client2.resetSession()
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("expected state file to be removed after reset, err=%v", err)
	}
}
