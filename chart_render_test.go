package yfgo

import (
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var updateChartGoldens = flag.Bool("update", false, "Update chart renderer golden PNGs")

func chartFixture() ChartResult {
	f := func(v float64) *float64 { return &v }
	v := func(v int64) *int64 { return &v }
	start := time.Date(2026, 1, 5, 14, 30, 0, 0, time.UTC).Unix()
	return ChartResult{
		Meta:      ChartMeta{Symbol: "AAPL", Range: "1mo", ExchangeTimezoneName: "America/New_York", ChartPreviousClose: f(100), PriceHint: v(2), DataGranularity: "1d"},
		Timestamp: []int64{start, start + 86400, start + 2*86400, start + 3*86400, start + 4*86400, start + 7*86400},
		Indicators: ChartIndicators{Quote: []ChartQuoteSeries{{
			Open:   []*float64{f(100), f(103), nil, f(102), f(99), f(104)},
			High:   []*float64{f(104), f(106), nil, f(104), f(104), f(106)},
			Low:    []*float64{f(98), f(101), nil, f(98), f(98), f(103)},
			Close:  []*float64{f(103), f(102), nil, f(99), f(104), f(104)},
			Volume: []*int64{v(1000), v(2500), nil, v(2000), v(1800), v(800)},
		}}},
	}
}

func TestRenderChartPNGGolden(t *testing.T) {
	for _, name := range []string{"candles_gaps", "single", "flat", "all_down", "intraday", "jp_timezone", "long_span", "empty", "quotes_array"} {
		t.Run(name, func(t *testing.T) {
			result := chartFixture()
			series := &result.Indicators.Quote[0]
			switch name {
			case "single":
				result.Timestamp = result.Timestamp[:1]
			case "flat":
				for i := range result.Timestamp {
					value := 100.0
					series.Open[i], series.High[i], series.Low[i], series.Close[i] = &value, &value, &value, &value
				}
			case "all_down":
				for i := range result.Timestamp {
					open, close, high, low := 105-float64(i), 103-float64(i), 106-float64(i), 102-float64(i)
					series.Open[i], series.Close[i], series.High[i], series.Low[i] = &open, &close, &high, &low
				}
			case "intraday", "jp_timezone":
				result.Meta.Range, result.Meta.DataGranularity = "1d", "1m"
				start := result.Timestamp[0]
				if name == "jp_timezone" {
					result.Meta.Symbol, result.Meta.ExchangeTimezoneName = "7203.T", "Asia/Tokyo"
					// Tokyo's next calendar date, well away from UTC's date.
					start = time.Date(2026, 1, 5, 23, 30, 0, 0, time.UTC).Unix()
				}
				for i := range result.Timestamp {
					result.Timestamp[i] = start + int64(i)*60
				}
			case "long_span":
				result.Meta.Range = "1y"
				start := result.Timestamp[0]
				for i := range result.Timestamp {
					result.Timestamp[i] = start + int64(i)*30*86400
				}
			case "empty":
				result.Timestamp, result.Indicators.Quote = nil, nil
			case "quotes_array":
				result.Quotes = chartRenderBars(result)
				result.Timestamp, result.Indicators.Quote = nil, nil
				result.Meta.ChartPreviousClose = nil
			}
			path := filepath.Join(t.TempDir(), "chart.png")
			if err := RenderChartPNG(result, RenderOptions{Path: path}); err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", "chart_"+name+".png")
			if *updateChartGoldens {
				if err := copyChartPNG(path, golden); err != nil {
					t.Fatal(err)
				}
			}
			got, want := decodeChartTestPNG(t, path), decodeChartTestPNG(t, golden)
			if got.Bounds() != want.Bounds() {
				t.Fatalf("bounds = %v, want %v", got.Bounds(), want.Bounds())
			}
			for y := 0; y < got.Bounds().Dy(); y++ {
				for x := 0; x < got.Bounds().Dx(); x++ {
					gr, gg, gb, ga := got.At(x, y).RGBA()
					wr, wg, wb, wa := want.At(x, y).RGBA()
					if gr != wr || gg != wg || gb != wb || ga != wa {
						t.Fatalf("pixel (%d,%d) differs; use make golden to update intentional changes", x, y)
					}
				}
			}
		})
	}
}

func decodeChartTestPNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestRenderChartPNGDimensionsAndAtomicWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chart.png")
	if err := RenderChartPNG(chartFixture(), RenderOptions{Path: path, Width: 640, Height: 320}); err != nil {
		t.Fatal(err)
	}
	if got := decodeChartTestPNG(t, path).Bounds(); got != image.Rect(0, 0, 640, 320) {
		t.Fatalf("bounds = %v", got)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []RenderOptions{{Path: path, Width: -1}, {Path: path, Height: 100}, {Path: path, Width: 100000}, {Path: "auto"}, {}} {
		if err := RenderChartPNG(chartFixture(), opts); err == nil {
			t.Fatalf("expected error for %+v", opts)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("failed render changed existing output")
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files leaked: %v, %v", files, err)
	}
}
