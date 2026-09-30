package yfgo

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"
	_ "time/tzdata" // Keep exchange-local labels available without system zoneinfo.

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// ChartRendererVersion invalidates cached PNGs when the renderer changes.
const ChartRendererVersion = "1"

// RenderOptions configures a PNG output file. Zero dimensions use 900 x 420.
// ChartPNG also accepts Path="auto" to choose a cached or temporary file.
type RenderOptions struct {
	Path   string
	Width  int
	Height int
}

var chartFont, chartFontError = opentype.Parse(goregular.TTF)

var (
	chartGreen = color.RGBA{22, 145, 91, 255}
	chartRed   = color.RGBA{213, 62, 62, 255}
	chartInk   = color.RGBA{55, 65, 75, 255}
	chartGrid  = color.RGBA{232, 236, 240, 255}
)

func (opts RenderOptions) dimensions() (RenderOptions, error) {
	if opts.Width == 0 {
		opts.Width = 900
	}
	if opts.Height == 0 {
		opts.Height = 420
	}
	if opts.Width < 320 || opts.Height < 200 || opts.Width > 8192 || opts.Height > 8192 || int64(opts.Width)*int64(opts.Height) > 32_000_000 {
		return opts, fmt.Errorf("chart dimensions must be at least 320 x 200, at most 8192 per side and 32 million pixels")
	}
	return opts, nil
}

// RenderChartPNG draws OHLC candles and volume from a ChartResult and writes
// opts.Path atomically. Text uses the embedded Go Regular font at 14 pixels
// (16 pixels for the title), with no dependency on installed system fonts.
func RenderChartPNG(result ChartResult, opts RenderOptions) error {
	opts, err := opts.dimensions()
	if err != nil {
		return err
	}
	if opts.Path == "" || opts.Path == "auto" {
		return fmt.Errorf("a PNG output path is required")
	}
	if chartFontError != nil {
		return chartFontError
	}
	face, err := opentype.NewFace(chartFont, &opentype.FaceOptions{Size: 14, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	defer face.Close()
	titleFace, err := opentype.NewFace(chartFont, &opentype.FaceOptions{Size: 16, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	defer titleFace.Close()

	img := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	label := func(x, y int, s string, ink color.Color, f font.Face) {
		d := font.Drawer{Dst: img, Src: image.NewUniform(ink), Face: f, Dot: fixed.P(x, y)}
		d.DrawString(s)
	}
	width := func(s string) int { return font.MeasureString(face, s).Ceil() }
	fill := func(rect image.Rectangle, ink color.Color) {
		draw.Draw(img, rect, image.NewUniform(ink), image.Point{}, draw.Src)
	}
	label(18, 28, result.Meta.Symbol+"  "+result.Meta.Range, chartInk, titleFace)

	bars := chartRenderBars(result)
	minimum, maximum := math.Inf(1), math.Inf(-1)
	var last *float64
	var lastColor color.RGBA
	var maxVolume int64
	for _, bar := range bars {
		if !validChartBar(bar) {
			continue
		}
		minimum = math.Min(minimum, math.Min(*bar.Low, math.Min(*bar.Open, *bar.Close)))
		maximum = math.Max(maximum, math.Max(*bar.High, math.Max(*bar.Open, *bar.Close)))
		last, lastColor = bar.Close, chartBarColor(bar)
		if bar.Volume != nil && *bar.Volume > maxVolume {
			maxVolume = *bar.Volume
		}
	}
	previous := result.Meta.ChartPreviousClose
	if last == nil {
		label(18, opts.Height/2, "No price data", chartInk, face)
		return writeChartFile(opts.Path, func(w io.Writer) error { return png.Encode(w, img) })
	}
	if finiteChartValue(previous) {
		minimum = math.Min(minimum, *previous)
		maximum = math.Max(maximum, *previous)
	}
	padding := (maximum - minimum) * 0.08
	if padding == 0 {
		padding = math.Max(math.Abs(minimum)*0.01, 0.01)
	}
	minimum, maximum = minimum-padding, maximum+padding
	step := chartNiceStep((maximum - minimum) / 4)
	minimum = math.Floor(minimum/step) * step
	maximum = math.Ceil(maximum/step) * step
	decimals := 2
	if result.Meta.PriceHint != nil {
		decimals = int(*result.Meta.PriceHint)
	}
	decimals = max(0, min(6, decimals))
	if step < 1 {
		decimals = max(decimals, min(6, int(math.Ceil(-math.Log10(step)))+1))
	}
	price := func(value float64) string { return strconv.FormatFloat(value, 'f', decimals, 64) }
	axisWidth := max(82, max(width(price(minimum)), width(price(maximum)))+18)
	left, right, top := 18, opts.Width-axisWidth, 55
	bottom := opts.Height - 36
	volumeTop := bottom - (bottom-top)/5
	priceBottom := volumeTop - 12
	priceY := func(value float64) int {
		return priceBottom - int(math.Round((value-minimum)/(maximum-minimum)*float64(priceBottom-top)))
	}
	for i := 0; i <= int(math.Round((maximum-minimum)/step)); i++ {
		value := minimum + float64(i)*step
		y := priceY(value)
		fill(image.Rect(left, y, right, y+1), chartGrid)
		label(right+8, y+5, price(value), chartInk, face)
	}
	fill(image.Rect(left, volumeTop-1, right, volumeTop), chartGrid)
	if finiteChartValue(previous) {
		y := priceY(*previous)
		for x := left; x < right; x += 10 {
			fill(image.Rect(x, y, min(x+5, right), y+1), color.RGBA{156, 163, 175, 255})
		}
	}
	spacing := float64(right-left) / float64(len(bars))
	bodyWidth := max(1, min(14, int(spacing*0.65)))
	barX := func(index int) int { return left + int((float64(index)+0.5)*spacing) }
	for i, bar := range bars {
		if !validChartBar(bar) {
			continue
		}
		x, ink := barX(i), chartBarColor(bar)
		fill(image.Rect(x, priceY(*bar.High), x+1, priceY(*bar.Low)+1), ink)
		y1, y2 := priceY(*bar.Open), priceY(*bar.Close)
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		fill(image.Rect(x-bodyWidth/2, y1, x-bodyWidth/2+bodyWidth, max(y1+1, y2)), ink)
		if bar.Volume != nil && *bar.Volume > 0 && maxVolume > 0 {
			height := int(math.Round(float64(*bar.Volume) / float64(maxVolume) * float64(bottom-volumeTop)))
			tint := color.RGBA{uint8((int(ink.R) + 510) / 3), uint8((int(ink.G) + 510) / 3), uint8((int(ink.B) + 510) / 3), 255}
			fill(image.Rect(x-bodyWidth/2, bottom-height, x-bodyWidth/2+bodyWidth, bottom), tint)
		}
	}

	loc := chartRenderLocation(result.Meta)
	layout := chartDateLayout(result.Meta.DataGranularity, bars[0].Date, bars[len(bars)-1].Date)
	count := min(5, len(bars))
	for i := 0; i < count; i++ {
		index := 0
		if count > 1 {
			index = int(math.Round(float64(i) * float64(len(bars)-1) / float64(count-1)))
		}
		s := time.Unix(bars[index].Date, 0).In(loc).Format(layout)
		x := max(left, min(right-width(s), barX(index)-width(s)/2))
		label(x, bottom+23, s, chartInk, face)
	}
	y := priceY(*last)
	tag := price(*last)
	fill(image.Rect(right+3, y-10, right+width(tag)+13, y+11), lastColor)
	label(right+8, y+5, tag, color.White, face)
	summary := price(*last)
	if finiteChartValue(previous) && *previous != 0 {
		summary += fmt.Sprintf("  %+.2f%%", (*last-*previous) / *previous * 100)
	}
	label(opts.Width-18-width(summary), 28, summary, lastColor, face)
	return writeChartFile(opts.Path, func(w io.Writer) error { return png.Encode(w, img) })
}

func finiteChartValue(value *float64) bool {
	return value != nil && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func validChartBar(bar ChartQuote) bool {
	return finiteChartValue(bar.Open) && finiteChartValue(bar.High) && finiteChartValue(bar.Low) && finiteChartValue(bar.Close) && *bar.High >= *bar.Low
}

func chartBarColor(bar ChartQuote) color.RGBA {
	if *bar.Close < *bar.Open {
		return chartRed
	}
	return chartGreen
}

func chartRenderBars(result ChartResult) []ChartQuote {
	if len(result.Timestamp) == 0 {
		return result.Quotes
	}
	bars := make([]ChartQuote, len(result.Timestamp))
	if len(result.Indicators.Quote) == 0 {
		return bars
	}
	series := result.Indicators.Quote[0]
	for i, ts := range result.Timestamp {
		bars[i].Date = ts
		if i < len(series.Open) {
			bars[i].Open = series.Open[i]
		}
		if i < len(series.High) {
			bars[i].High = series.High[i]
		}
		if i < len(series.Low) {
			bars[i].Low = series.Low[i]
		}
		if i < len(series.Close) {
			bars[i].Close = series.Close[i]
		}
		if i < len(series.Volume) {
			bars[i].Volume = series.Volume[i]
		}
	}
	return bars
}

func chartNiceStep(value float64) float64 {
	power := math.Pow(10, math.Floor(math.Log10(value)))
	for _, factor := range []float64{1, 2, 2.5, 5, 10} {
		if value <= factor*power {
			return factor * power
		}
	}
	return 10 * power
}

func chartDateLayout(interval string, first, last int64) string {
	span := time.Duration(last-first) * time.Second
	switch {
	case span < 48*time.Hour && interval != "1d" && interval != "5d" && interval != "1wk" && interval != "1mo" && interval != "3mo":
		return "15:04"
	case span < 120*24*time.Hour:
		return "Jan 2"
	default:
		return "Jan 2006"
	}
}

func chartRenderLocation(meta ChartMeta) *time.Location {
	if meta.ExchangeTimezoneName != "" {
		if loc, err := time.LoadLocation(meta.ExchangeTimezoneName); err == nil {
			return loc
		}
	}
	return time.FixedZone(meta.Timezone, int(meta.GmtOffset))
}

func writeChartFile(path string, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".yf-chart-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := write(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
