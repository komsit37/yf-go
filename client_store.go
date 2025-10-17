package yfgo

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type clientStore struct {
	statePath string
}

type clientState struct {
	Crumb   string                    `json:"crumb"`
	Cookies map[string][]storedCookie `json:"cookies"`
	SavedAt time.Time                 `json:"savedAt"`
}

type storedCookie struct {
	Name     string        `json:"name"`
	Value    string        `json:"value"`
	Path     string        `json:"path"`
	Domain   string        `json:"domain"`
	Expires  time.Time     `json:"expires"`
	MaxAge   int           `json:"maxAge"`
	Secure   bool          `json:"secure"`
	HTTPOnly bool          `json:"httpOnly"`
	SameSite http.SameSite `json:"sameSite"`
}

func (c *Client) initStore() {
	store, err := newClientStore()
	if err != nil {
		return
	}
	c.store = store
	c.restoreState()
}

func newClientStore() (*clientStore, error) {
	dir, err := resolveStateDir()
	if err != nil || dir == "" {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &clientStore{statePath: filepath.Join(dir, "state.json")}, nil
}

func resolveStateDir() (string, error) {
	if home := os.Getenv("YF_HOME"); home != "" {
		return home, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if userHome == "" {
		return "", errors.New("user home directory not found")
	}
	return filepath.Join(userHome, ".yf"), nil
}

func (c *Client) restoreState() {
	if c.store == nil {
		return
	}
	state, err := c.store.Load()
	if err != nil || state == nil {
		return
	}
	if state.Crumb != "" {
		c.crumb = state.Crumb
	}
	if len(state.Cookies) == 0 {
		return
	}
	for rawURL, stored := range state.Cookies {
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		c.http.Jar.SetCookies(u, toHTTPCookies(stored))
	}
	c.sessionWarmed = true
}

func (c *Client) saveState() {
	if c.store == nil || c.crumb == "" {
		return
	}
	state := clientState{
		Crumb:   c.crumb,
		Cookies: make(map[string][]storedCookie),
		SavedAt: time.Now().UTC(),
	}
	for _, rawURL := range crumbCookieHosts {
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		cookies := c.http.Jar.Cookies(u)
		if len(cookies) == 0 {
			continue
		}
		state.Cookies[rawURL] = fromHTTPCookies(cookies)
	}
	_ = c.store.Save(state)
}

func (c *Client) clearState() {
	if c.store == nil {
		return
	}
	_ = c.store.Clear()
}

func (s *clientStore) Load() (*clientState, error) {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var state clientState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *clientStore) Save(state clientState) error {
	if state.Crumb == "" {
		return nil
	}
	tmpPath := s.statePath + ".tmp"
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.statePath)
}

func (s *clientStore) Clear() error {
	err := os.Remove(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func fromHTTPCookies(cookies []*http.Cookie) []storedCookie {
	out := make([]storedCookie, 0, len(cookies))
	for _, c := range cookies {
		out = append(out, storedCookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			MaxAge:   c.MaxAge,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
			SameSite: c.SameSite,
		})
	}
	return out
}

func toHTTPCookies(cookies []storedCookie) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		out = append(out, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			MaxAge:   c.MaxAge,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
			SameSite: c.SameSite,
		})
	}
	return out
}

var crumbCookieHosts = []string{
	"https://fc.yahoo.com/",
	"https://finance.yahoo.com/",
	"https://query1.finance.yahoo.com/",
}
