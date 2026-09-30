package yfgo

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sheetFixture() map[string]any {
	f := func(value float64) map[string]any { return map[string]any{"raw": value} }
	return map[string]any{
		"assetProfile":         map[string]any{"sector": "Consumer Cyclical", "industry": "Auto Manufacturers", "country": "Japan", "fullTimeEmployees": 380793, "website": "https://global.toyota", "longBusinessSummary": "Toyota designs and manufactures passenger cars and commercial vehicles worldwide."},
		"price":                map[string]any{"longName": "Toyota Motor Corporation", "currency": "JPY", "exchangeName": "Tokyo", "marketCap": f(45.1e12), "regularMarketPrice": f(150), "averageDailyVolume3Month": f(20.5e6)},
		"summaryDetail":        map[string]any{"trailingPE": f(9.1), "fiftyTwoWeekLow": f(100), "fiftyTwoWeekHigh": f(200), "dividendYield": f(0.029), "payoutRatio": f(0), "exDividendDate": f(1790640000)},
		"defaultKeyStatistics": map[string]any{"priceToBook": f(1.05), "forwardPE": f(10.2)},
		"financialData":        map[string]any{"returnOnEquity": f(0.12), "financialCurrency": "USD", "totalCash": f(2.45e9), "debtToEquity": f(65.838), "revenueGrowth": f(-0.042), "earningsGrowth": f(0.02)},
		"calendarEvents":       map[string]any{"earnings": map[string]any{"earningsDate": []any{f(1793836800)}}},
	}
}

func TestSheetFinancial(t *testing.T) {
	for _, tt := range []struct {
		value                any
		kind, currency, want string
	}{
		{45.1e12, "money", "JPY", "¥45.1T"}, {2.45e9, "money", "USD", "$2.45B"},
		{-2.45e9, "money", "USD", "−$2.45B"}, {0.0, "money", "USD", "$0"},
		{100.0, "price", "JPY", "¥100"}, {2345.6, "price", "USD", "$2,345.60"},
		{9.1, "ratio", "", "9.10×"}, {0.029, "percent", "", "2.9%"},
		{65.838, "debtPercent", "", "65.8%"}, {-0.042, "change", "", "−4.2%"},
		{0.02, "change", "", "+2.0%"}, {-0.00001, "change", "", "0.0%"},
		{20.5e6, "compact", "", "20.5M"}, {380793.0, "integer", "", "380,793"},
		{nil, "money", "USD", ""}, {map[string]any{"fmt": "N/A"}, "ratio", "", ""},
		{map[string]any{"fmt": "9.10"}, "ratio", "", "9.10"},
		{map[string]any{"raw": 0.025, "fmt": "wrong"}, "percent", "", "2.5%"},
	} {
		if got := sheetFinancial(tt.value, tt.kind, tt.currency); got != tt.want {
			t.Errorf("sheetFinancial(%v, %s, %s) = %q, want %q", tt.value, tt.kind, tt.currency, got, tt.want)
		}
	}
}

func TestParseSheetData(t *testing.T) {
	data, err := ParseSheetData(sheetFixture())
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, row := range append(data.Metrics, data.Company...) {
		rows[row.Label] = row.Value
	}
	for label, want := range map[string]string{"Mkt cap": "¥45.1T", "P/E": "9.10×", "Fwd P/E": "10.20×", "D/E": "65.8%", "Rev grow": "−4.2%", "Cash": "$2.45B", "Payout": "0.0%", "52w pos": "50.0%", "Employees": "380,793", "Website": "global.toyota", "Earnings": "2026-11-05", "Ex-div": "2026-09-29"} {
		if rows[label] != want {
			t.Errorf("%s = %q, want %q", label, rows[label], want)
		}
	}
	if _, exists := rows["Debt"]; exists {
		t.Fatal("missing values must be omitted")
	}
	if data.Title != "Toyota Motor Corporation" || data.Summary == "" || data.Website != "https://global.toyota" {
		t.Fatal("missing company text")
	}
	empty, err := ParseSheetData(map[string]any{})
	if err != nil || len(empty.Metrics) != 0 || len(empty.Company) != 0 {
		t.Fatalf("empty=%+v error=%v", empty, err)
	}
}

func TestRenderSheetPNGInvalidOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sheet.png")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, opts := range []SheetRenderOptions{
		{RenderOptions: RenderOptions{Path: path}, Theme: "purple"},
		{RenderOptions: RenderOptions{Path: path}, FontSize: math.NaN()},
		{RenderOptions: RenderOptions{Path: path}, FontSize: -1},
		{RenderOptions: RenderOptions{Path: path, Width: -1}},
		{RenderOptions: RenderOptions{Path: "auto"}},
		{},
	} {
		if err := RenderSheetPNG(chartFixture(), SheetData{}, opts); err == nil {
			t.Fatalf("invalid options accepted: %+v", opts)
		}
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != "existing" {
		t.Fatal("invalid render changed an existing file")
	}
}

func TestRenderSheetPNGGolden(t *testing.T) {
	for _, name := range []string{"light", "dark", "empty", "long_title", "small_font"} {
		t.Run(name, func(t *testing.T) {
			data, err := ParseSheetData(sheetFixture())
			if err != nil {
				t.Fatal(err)
			}
			result := chartFixture()
			result.Meta.Symbol, result.Meta.Currency, result.Meta.Range = "7203.T", "JPY", "1y"
			path := filepath.Join(t.TempDir(), "sheet.png")
			opts := SheetRenderOptions{RenderOptions: RenderOptions{Path: path}}
			switch name {
			case "dark":
				opts.Theme = "dark"
			case "empty":
				data = SheetData{}
				result.Timestamp, result.Indicators.Quote = nil, nil
			case "long_title":
				data.Title = "ORIGINAL ENGINEERING CONSULTANTS CO.,LTD."
				data.Summary = strings.Repeat("A long company summary with financial information. ", 8)
			case "small_font":
				opts.FontSize = 18
			}
			if err := RenderSheetPNG(result, data, opts); err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", "sheet_"+name+".png")
			if *updateChartGoldens {
				if err := copyChartPNG(path, golden); err != nil {
					t.Fatal(err)
				}
			}
			got, want := decodeChartTestPNG(t, path), decodeChartTestPNG(t, golden)
			if got.Bounds() != want.Bounds() {
				t.Fatalf("bounds=%v want=%v", got.Bounds(), want.Bounds())
			}
			for y := 0; y < got.Bounds().Dy(); y++ {
				for x := 0; x < got.Bounds().Dx(); x++ {
					gr, gg, gb, ga := got.At(x, y).RGBA()
					wr, wg, wb, wa := want.At(x, y).RGBA()
					if gr != wr || gg != wg || gb != wb || ga != wa {
						t.Fatalf("pixel (%d,%d) differs; use make golden", x, y)
					}
				}
			}
			_, _, _, alpha := got.At(got.Bounds().Dx()-1, 0).RGBA()
			if alpha != 0 {
				t.Fatal("text background must be transparent")
			}
			_, _, _, alpha = got.At(0, got.Bounds().Dy()/2).RGBA()
			if name == "empty" && alpha != 65535 {
				t.Fatal("chart background must remain opaque")
			}
		})
	}
}

func TestSheetPNGCache(t *testing.T) {
	t.Setenv("YF_HOME", t.TempDir())
	dir := t.TempDir()
	store, err := NewFileCacheStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	chartCalls, summaryCalls := 0, 0
	newClient := func() *Client {
		client := NewClient(WithCacheStore(store))
		client.sessionWarmed, client.crumb = true, "crumb"
		client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.HasSuffix(req.URL.Path, "/7203.T") {
				t.Fatalf("symbol path: %s", req.URL.Path)
			}
			var payload any
			if strings.Contains(req.URL.Path, "/chart/") {
				chartCalls++
				payload = map[string]any{"chart": map[string]any{"result": []ChartResult{chartFixture()}, "error": nil}}
			} else {
				summaryCalls++
				payload = map[string]any{"quoteSummary": map[string]any{"result": []any{sheetFixture()}, "error": nil}}
			}
			body, _ := json.Marshal(payload)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
		})
		return client
	}
	ctx := context.Background()
	query := ChartOptions{Range: "1y", Interval: "1d", ReturnType: "object"}
	opts := SheetRenderOptions{RenderOptions: RenderOptions{Path: "auto"}}
	client := newClient()
	// Existing chart and qs cache entries are reused by the first sheet request.
	if _, err := client.ChartTyped(ctx, "JP:7203", query); err != nil {
		t.Fatal(err)
	}
	if _, err := client.QuoteSummary(ctx, "JP:7203", sheetModules); err != nil {
		t.Fatal(err)
	}
	path, err := client.SheetPNG(ctx, "JP:7203", query, opts)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	client = newClient()
	second, err := client.SheetPNG(ctx, "7203.T", query, opts)
	secondInfo, _ := os.Stat(second)
	if err != nil || second != path || chartCalls != 1 || summaryCalls != 1 || !os.SameFile(info, secondInfo) {
		t.Fatalf("cache miss: chart=%d summary=%d path=%s error=%v", chartCalls, summaryCalls, second, err)
	}
	if filepath.Dir(path) != filepath.Join(dir, "render") {
		t.Fatalf("cache path=%s", path)
	}
	// An expired fundamental module refreshes fundamentals but keeps daily bars.
	key := cacheKeyQuoteSummaryModule("7203.T", ModulePrice)
	entry, _, _ := store.Get(ctx, key)
	entry.StoredAt = time.Now().Add(-6 * time.Minute)
	if err := store.Set(ctx, key, entry); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SheetPNG(ctx, "JP:7203", query, opts); err != nil || chartCalls != 1 || summaryCalls != 2 {
		t.Fatalf("module expiry: chart=%d summary=%d error=%v", chartCalls, summaryCalls, err)
	}
	for _, change := range []SheetRenderOptions{
		{RenderOptions: RenderOptions{Path: "auto"}, Theme: "dark"},
		{RenderOptions: RenderOptions{Path: "auto"}, FontSize: 18},
		{RenderOptions: RenderOptions{Path: "auto"}, Title: "Custom title"},
	} {
		changed, err := client.SheetPNG(ctx, "JP:7203", query, change)
		if err != nil || changed == path || chartCalls != 1 || summaryCalls != 2 {
			t.Fatalf("render variant: %s error=%v", changed, err)
		}
	}
	explicit := filepath.Join(t.TempDir(), "out.png")
	opts.Path = explicit
	if output, err := client.SheetPNG(ctx, "JP:7203", query, opts); err != nil || output != explicit {
		t.Fatalf("explicit path=%s error=%v", output, err)
	}
	if _, err := client.SheetPNG(WithCacheOptions(ctx, ForceRefresh()), "JP:7203", query, opts); err != nil || chartCalls != 2 || summaryCalls != 3 {
		t.Fatalf("refresh error=%v", err)
	}
	opts.Path = "auto"
	for _, option := range []RequestOption{BypassCache(), CacheTTL(0)} {
		uncached, err := client.SheetPNG(WithCacheOptions(ctx, option), "JP:7203", query, opts)
		if err != nil || uncached == path || filepath.Dir(uncached) == filepath.Join(dir, "render") {
			t.Fatalf("bypass path=%s error=%v", uncached, err)
		}
		defer os.Remove(uncached)
	}
	if chartCalls != 4 || summaryCalls != 5 {
		t.Fatalf("bypass requests chart=%d summary=%d", chartCalls, summaryCalls)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if config, err := png.DecodeConfig(f); err != nil || config.Width != 800 || config.Height <= 400 {
		t.Fatalf("sheet dimensions=%+v error=%v", config, err)
	}
}
