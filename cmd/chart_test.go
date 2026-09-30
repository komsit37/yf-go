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

type chartTransportFunc func(*http.Request) (*http.Response, error)

func (f chartTransportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChartRenderCLIContract(t *testing.T) {
	t.Setenv("YF_HOME", t.TempDir())
	for _, key := range []string{"YF_CACHE_TTL", "YF_NO_CACHE", "YF_FORCE_REFRESH", "YF_CACHE_DIR", "YF_CHART_RENDER"} {
		t.Setenv(key, "")
	}
	calls := 0
	previousTransport := http.DefaultTransport
	http.DefaultTransport = chartTransportFunc(func(req *http.Request) (*http.Response, error) {
		body := "crumb"
		if strings.Contains(req.URL.Path, "/v8/finance/chart/") {
			calls++
			if !strings.HasSuffix(req.URL.Path, "/7203.T") || req.URL.Query().Get("interval") != "1d" || req.URL.Query().Get("range") != "6mo" {
				t.Fatalf("unexpected chart request: %s", req.URL)
			}
			body = `{"chart":{"result":[{"meta":{"symbol":"7203.T","range":"6mo","dataGranularity":"1d","exchangeTimezoneName":"Asia/Tokyo","chartPreviousClose":100},"timestamp":[1767628800],"indicators":{"quote":[{"open":[100],"high":[104],"low":[99],"close":[103],"volume":[1000]}]}}],"error":null}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	resetFlags := func() {
		for _, name := range []string{"render", "width", "height", "range", "interval", "plot", "format", "force-refresh", "no-cache"} {
			flag := chartCmd.Flags().Lookup(name)
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
		resetFlags()
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	run := func(extra ...string) (string, error) {
		resetFlags()
		var stdout, stderr bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		args := []string{"chart", "JP:7203", "--range", "6mo", "--interval", "1d", "--width", "640", "--height", "320"}
		rootCmd.SetArgs(append(args, extra...))
		err := rootCmd.Execute()
		return stdout.String(), err
	}
	first, err := run("--render", "auto", "--format", "ignored", "--plot")
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSuffix(first, "\n")
	if first != path+"\n" || !filepath.IsAbs(path) || strings.Contains(path, "\n") || calls != 1 {
		t.Fatalf("stdout=%q requests=%d", first, calls)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(f)
	f.Close()
	if err != nil || config.Width != 640 || config.Height != 320 {
		t.Fatalf("PNG config=%+v error=%v", config, err)
	}
	if second, err := run("--render", "auto", "--format", "json"); err != nil || second != first || calls != 1 {
		t.Fatalf("cache hit: stdout=%q requests=%d error=%v", second, calls, err)
	}
	explicit := filepath.Join(t.TempDir(), "preview.png")
	if got, err := run("--render", explicit); err != nil || got != explicit+"\n" || calls != 1 {
		t.Fatalf("explicit render: stdout=%q requests=%d error=%v", got, calls, err)
	}
	if _, err := run("--render", "auto", "--force-refresh"); err != nil || calls != 2 {
		t.Fatalf("force refresh: requests=%d error=%v", calls, err)
	}
	uncached, err := run("--render", "auto", "--no-cache")
	if err != nil || calls != 3 || uncached == first {
		t.Fatalf("bypass: stdout=%q requests=%d error=%v", uncached, calls, err)
	}
	defer os.Remove(strings.TrimSpace(uncached))
	for _, args := range [][]string{{"--render"}, {"--render", ""}, {"--render", "auto", "--width", "-1"}} {
		if _, err := run(args...); err == nil {
			t.Fatalf("expected invalid flags to fail: %v", args)
		}
	}
}
