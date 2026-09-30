package yfgo

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// SheetRendererVersion invalidates cached sheets when their layout changes.
const SheetRendererVersion = "3"

// SheetRenderOptions controls a security sheet. Width and Height specify the
// chart pane (default 800 x 400); the PNG grows vertically to fit the text.
// FontSize is in PNG pixels (default 22, or 11 when displayed at half size).
// Text has a transparent background. Theme selects readable light/dark ink.
type SheetRenderOptions struct {
	RenderOptions
	FontSize float64
	Theme    string
	Title    string
}

func (opts SheetRenderOptions) dimensions() (SheetRenderOptions, error) {
	if opts.Width == 0 {
		opts.Width = 800
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	var err error
	opts.RenderOptions, err = opts.RenderOptions.dimensions()
	if err != nil {
		return opts, err
	}
	if opts.FontSize == 0 {
		opts.FontSize = 22
	}
	if math.IsNaN(opts.FontSize) || math.IsInf(opts.FontSize, 0) || opts.FontSize < 6 || opts.FontSize > 96 {
		return opts, fmt.Errorf("sheet font size must be between 6 and 96 pixels")
	}
	if opts.Theme == "" {
		opts.Theme = "light"
	}
	if opts.Theme != "light" && opts.Theme != "dark" {
		return opts, fmt.Errorf("sheet theme must be light or dark")
	}
	return opts, nil
}

// RenderSheetPNG draws a title, candlestick chart, three aligned columns of
// financial metrics, company details and summary into one atomic PNG.
func RenderSheetPNG(result ChartResult, data SheetData, opts SheetRenderOptions) error {
	opts, err := opts.dimensions()
	if err != nil {
		return err
	}
	if opts.Path == "" || opts.Path == "auto" {
		return fmt.Errorf("a PNG output path is required")
	}
	chart, err := renderChartImage(result, opts.RenderOptions)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(chartFont, &opentype.FaceOptions{Size: opts.FontSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	defer face.Close()
	lineHeight := face.Metrics().Height.Ceil()
	ascent := face.Metrics().Ascent.Ceil()
	ink, muted := color.Color(chartInk), color.Color(color.RGBA{110, 115, 120, 255})
	if opts.Theme == "dark" {
		ink, muted = color.RGBA{230, 230, 230, 255}, color.RGBA{170, 170, 170, 255}
	}
	title := opts.Title
	if title == "" {
		title = data.Title
	}
	if title == "" {
		title = result.Meta.LongName
	}
	if title == "" {
		title = result.Meta.Symbol
	}
	title += " · " + result.Meta.Range
	lastLabel := ""
	lastInk := ink
	for _, bar := range chartRenderBars(result) {
		if !validChartBar(bar) {
			continue
		}
		lastLabel = sheetFinancial(*bar.Close, "price", result.Meta.Currency)
		lastInk = chartBarColor(bar)
		if finiteChartValue(result.Meta.ChartPreviousClose) && *result.Meta.ChartPreviousClose != 0 {
			change := (*bar.Close - *result.Meta.ChartPreviousClose) / *result.Meta.ChartPreviousClose
			lastLabel += " " + sheetFinancial(change, "change", "")
			if change < 0 {
				lastInk = chartRed
			} else if change > 0 {
				lastInk = chartGreen
			}
		}
	}
	lastWidth := font.MeasureString(face, lastLabel).Ceil()
	titleWidth := opts.Width
	if lastLabel != "" && lastWidth < opts.Width/2 {
		titleWidth -= lastWidth + 8
	} else {
		lastLabel = ""
	}
	titleLines := wrapSheetText(face, title, titleWidth)
	chartTop := len(titleLines) * lineHeight
	// Keep the price pane and date labels, trimming the old standalone header.
	chartBounds := image.Rect(0, 38, opts.Width, opts.Height-8)
	chartBottom := chartTop + chartBounds.Dy()
	companyLines := []string{}
	for _, row := range data.Company {
		companyLines = append(companyLines, wrapSheetText(face, row.Label+" "+row.Value, opts.Width)...)
	}
	summaryLines := []string{}
	if data.Summary != "" {
		summaryLines = wrapSheetText(face, "About "+data.Summary, opts.Width)
	}
	height := chartBottom + ((len(data.Metrics)+2)/3+len(companyLines)+len(summaryLines))*lineHeight
	if height > 8192 || int64(opts.Width)*int64(height) > 32_000_000 {
		return fmt.Errorf("sheet content exceeds 8192 pixels high or 32 million pixels")
	}
	img := image.NewRGBA(image.Rect(0, 0, opts.Width, height))
	label := func(dst draw.Image, x, y int, text string, color color.Color) {
		d := font.Drawer{Dst: dst, Src: image.NewUniform(color), Face: face, Dot: fixed.P(x, y+ascent)}
		d.DrawString(text)
	}
	y := 0
	for _, line := range titleLines {
		label(img, 0, y, line, ink)
		y += lineHeight
	}
	label(img, opts.Width-lastWidth, 0, lastLabel, lastInk)
	draw.Draw(img, image.Rect(0, chartTop, opts.Width, chartBottom), chart, chartBounds.Min, draw.Src)
	y = chartBottom
	for index, row := range data.Metrics {
		column := index % 3
		left, right := column*opts.Width/3, (column+1)*opts.Width/3-6
		valueWidth := font.MeasureString(face, row.Value).Ceil()
		cell := img.SubImage(image.Rect(left, y, right, y+lineHeight)).(*image.RGBA)
		label(cell, left, y, sheetEllipsis(face, row.Label, max(0, right-left-valueWidth-4)), muted)
		valueInk := ink
		if strings.HasPrefix(row.Value, "+") {
			valueInk = chartGreen
		}
		if strings.HasPrefix(row.Value, "−") {
			valueInk = chartRed
		}
		label(cell, right-valueWidth, y, row.Value, valueInk)
		if column == 2 || index == len(data.Metrics)-1 {
			y += lineHeight
		}
	}
	for _, line := range append(companyLines, summaryLines...) {
		label(img, 0, y, line, ink)
		y += lineHeight
	}
	return writeChartFile(opts.Path, func(w io.Writer) error { return encodeSheetPNG(w, img, data.Website) })
}

func wrapSheetText(face font.Face, text string, width int) []string {
	lines := []string{}
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && font.MeasureString(face, line+" "+word).Ceil() > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		for _, char := range word {
			if line != "" && font.MeasureString(face, line+string(char)).Ceil() > width {
				lines = append(lines, line)
				line = ""
			}
			line += string(char)
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func sheetEllipsis(face font.Face, text string, width int) string {
	if font.MeasureString(face, text).Ceil() <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if font.MeasureString(face, string(runes)+"…").Ceil() <= width {
			return string(runes) + "…"
		}
	}
	return ""
}
