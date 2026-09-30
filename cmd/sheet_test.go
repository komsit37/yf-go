package cmd

import (
	"bytes"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheetCLIContract(t *testing.T) {
	t.Setenv("YF_HOME", t.TempDir())
	for _, key := range []string{"YF_CACHE_TTL", "YF_NO_CACHE", "YF_FORCE_REFRESH", "YF_CACHE_DIR", "YF_SHEET_RENDER"} {
		t.Setenv(key, "")
	}
	chartCalls, summaryCalls := 0, 0
	expectedRange, expectedInterval, expectedPrePost := "1y", "1d", "true"
	previousTransport := http.DefaultTransport
	http.DefaultTransport = chartTransportFunc(func(req *http.Request) (*http.Response, error) {
		body := "crumb"
		switch {
		case strings.Contains(req.URL.Path, "/v8/finance/chart/"):
			chartCalls++
			if !strings.HasSuffix(req.URL.Path, "/7203.T") || req.URL.Query().Get("range") != expectedRange || req.URL.Query().Get("interval") != expectedInterval || req.URL.Query().Get("includePrePost") != expectedPrePost {
				t.Fatalf("chart URL=%s", req.URL)
			}
			body = `{"chart":{"result":[{"meta":{"symbol":"7203.T","currency":"JPY","range":"1y","chartPreviousClose":100},"timestamp":[1767628800],"indicators":{"quote":[{"open":[100],"high":[104],"low":[99],"close":[103]}]}}],"error":null}}`
		case strings.Contains(req.URL.Path, "/v10/finance/quoteSummary/"):
			summaryCalls++
			if !strings.HasSuffix(req.URL.Path, "/7203.T") {
				t.Fatalf("summary URL=%s", req.URL)
			}
			// Missing modules must also be cached, rather than refetched per row.
			body = `{"quoteSummary":{"result":[{"price":{"longName":"Toyota","currency":"JPY","marketCap":{"raw":45100000000000}},"summaryDetail":{"trailingPE":{"raw":9.1}}}],"error":null}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	reset := func() {
		for _, name := range []string{"render", "width", "height", "range", "interval", "include-pre-post", "period1", "period2", "font-size", "theme", "title", "format", "force-refresh", "no-cache", "cache-ttl"} {
			flag := sheetCmd.Flags().Lookup(name)
			if flag == nil {
				flag = rootCmd.PersistentFlags().Lookup(name)
			}
			if err := flag.Value.Set(flag.DefValue); err != nil {
				t.Fatal(err)
			}
			flag.Changed = false
		}
	}
	t.Cleanup(func() {
		http.DefaultTransport = previousTransport
		reset()
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	run := func(extra ...string) (string, error) {
		reset()
		var stdout, stderr bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs(append([]string{"sheet", "JP:7203", "--range", "1y", "--interval", "1d", "--width", "800", "--height", "400"}, extra...))
		err := rootCmd.Execute()
		return stdout.String(), err
	}
	first, err := run("--render", "auto", "--format", "ignored")
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSuffix(first, "\n")
	if first != path+"\n" || !filepath.IsAbs(path) || strings.Contains(path, "\n") || chartCalls != 1 || summaryCalls != 1 {
		t.Fatalf("stdout=%q chart=%d summary=%d", first, chartCalls, summaryCalls)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(f)
	f.Close()
	if err != nil || config.Width != 800 || config.Height <= 400 {
		t.Fatalf("config=%+v error=%v", config, err)
	}
	info, _ := os.Stat(path)
	if second, err := run(); err != nil || second != first || chartCalls != 1 || summaryCalls != 1 {
		t.Fatalf("cache miss: stdout=%q error=%v", second, err)
	}
	secondInfo, _ := os.Stat(path)
	if !os.SameFile(info, secondInfo) {
		t.Fatal("cached sheet was rerendered")
	}
	explicit := filepath.Join(t.TempDir(), "sheet.png")
	if result, err := run("--render", explicit); err != nil || result != explicit+"\n" {
		t.Fatalf("explicit=%q error=%v", result, err)
	}
	if _, err := run("--force-refresh"); err != nil || chartCalls != 2 || summaryCalls != 2 {
		t.Fatalf("refresh chart=%d summary=%d error=%v", chartCalls, summaryCalls, err)
	}
	uncached, err := run("--no-cache")
	if err != nil || uncached == first || chartCalls != 3 || summaryCalls != 3 {
		t.Fatalf("bypass=%q error=%v", uncached, err)
	}
	defer os.Remove(strings.TrimSpace(uncached))
	for _, extra := range [][]string{{"--render"}, {"--render", ""}, {"--theme", "purple"}, {"--font-size", "2"}, {"--width", "-1"}, {"--period1", "bad-date"}} {
		if _, err := run(extra...); err == nil {
			t.Fatalf("expected invalid flags to fail: %v", extra)
		}
	}
	expectedRange, expectedInterval, expectedPrePost = "1d", "1m", "false"
	day, err := run("--range", "1d", "--interval", "1m", "--include-pre-post=false")
	if err != nil || day == first || chartCalls != 4 || summaryCalls != 3 {
		t.Fatalf("current trading day: stdout=%q chart=%d summary=%d error=%v", day, chartCalls, summaryCalls, err)
	}
	if cached, err := run("--range", "1d", "--interval", "1m", "--include-pre-post=false"); err != nil || cached != day || chartCalls != 4 || summaryCalls != 3 {
		t.Fatalf("intraday cache miss: stdout=%q error=%v", cached, err)
	}
}
