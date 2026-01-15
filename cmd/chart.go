package cmd

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	yfgo "github.com/komsit37/yf-go"
)

// chartCmd fetches time-series data from v8/finance/chart.
var chartCmd = &cobra.Command{
	Use:   "chart <symbol>",
	Short: "Get price chart/time-series data for a symbol",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		symbols := parseSymbols(args)
		if len(symbols) == 0 {
			return fmt.Errorf("symbol is required")
		}
		if len(symbols) > 1 {
			return fmt.Errorf("chart only supports a single symbol; got %d", len(symbols))
		}
		symbol := symbols[0]

		period1Raw := viper.GetString("chart-period1")
		if strings.TrimSpace(period1Raw) == "" {
			period1Raw = viper.GetString("chart-start")
		}
		period1, err := parseChartTimestamp("period1", period1Raw)
		if err != nil {
			return err
		}
		period2Raw := viper.GetString("chart-period2")
		if strings.TrimSpace(period2Raw) == "" {
			period2Raw = viper.GetString("chart-end")
		}
		period2, err := parseChartTimestamp("period2", period2Raw)
		if err != nil {
			return err
		}

		ret := strings.ToLower(strings.TrimSpace(viper.GetString("chart-return")))
		switch ret {
		case "":
			ret = "array"
		case "array", "object":
		default:
			return fmt.Errorf("invalid --return value %q (allowed: array, object)", ret)
		}

		includePrePost := viper.GetBool("chart-include-pre-post")
		useYfid := viper.GetBool("chart-use-yfid")

		rangeVal := viper.GetString("chart-range")
		if (period1 != nil || period2 != nil) && !cmd.Flags().Changed("range") {
			rangeVal = ""
		}

		opts := yfgo.ChartOptions{
			Interval:       viper.GetString("chart-interval"),
			Range:          rangeVal,
			Period1:        period1,
			Period2:        period2,
			IncludePrePost: &includePrePost,
			Events:         viper.GetString("chart-events"),
			Lang:           viper.GetString("chart-lang"),
			UseYfid:        &useYfid,
			ReturnType:     ret,
		}

		// Argument parsing completed; suppress usage on runtime errors.
		cmd.SilenceUsage = true

		ctx := requestContext(cmd)
		format := viper.GetString("format")
		plot := viper.GetBool("chart-plot")
		if plot && format != "table" {
			return fmt.Errorf("--plot is only supported with --format table")
		}
		switch format {
		case "json":
			optsJSON := opts
			data, err := yfgo.DefaultAPI.Chart(ctx, symbol, optsJSON)
			if err != nil {
				return err
			}
			return printJSON(data, viper.GetBool("pretty"))
		case "table":
			cols, err := parseChartColumns(viper.GetString("chart-columns"))
			if err != nil {
				return err
			}
			optsTable := opts
			optsTable.ReturnType = "object"
			typed, err := yfgo.DefaultAPI.ChartTyped(ctx, symbol, optsTable)
			if err != nil {
				return err
			}
			renderChartTable(os.Stdout, typed, cols, plot)
			return nil
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}
	},
}

func init() {
	rootCmd.AddCommand(chartCmd)

	chartCmd.Flags().StringP("interval", "i", "1mo", "Data interval (1m, 2m, 5m, 15m, 30m, 60m, 90m, 1h, 1d, 5d, 1wk, 1mo, 3mo)")
	_ = viper.BindPFlag("chart-interval", chartCmd.Flags().Lookup("interval"))

	chartCmd.Flags().StringP("range", "r", "1y", "Date range (e.g. 1d, 5d, 1mo, 3mo, 6mo, 1y, 5y, ytd, max). Use period flags for a custom window.")
	_ = viper.BindPFlag("chart-range", chartCmd.Flags().Lookup("range"))

	chartCmd.Flags().String("period1", "", "Custom range start (unix seconds or date: YYYY-MM-DD, RFC3339).")
	_ = viper.BindPFlag("chart-period1", chartCmd.Flags().Lookup("period1"))

	chartCmd.Flags().String("period2", "", "Custom range end (unix seconds or date: YYYY-MM-DD, RFC3339). Defaults to now when omitted.")
	_ = viper.BindPFlag("chart-period2", chartCmd.Flags().Lookup("period2"))

	chartCmd.Flags().StringP("start", "s", "", "Alias for --period1 (custom range start)")
	_ = viper.BindPFlag("chart-start", chartCmd.Flags().Lookup("start"))

	chartCmd.Flags().StringP("end", "e", "", "Alias for --period2 (custom range end)")
	_ = viper.BindPFlag("chart-end", chartCmd.Flags().Lookup("end"))

	chartCmd.Flags().Bool("include-pre-post", true, "Include pre/post market data")
	_ = viper.BindPFlag("chart-include-pre-post", chartCmd.Flags().Lookup("include-pre-post"))

	chartCmd.Flags().Bool("use-yfid", true, "Use Yahoo Finance YFID header")
	_ = viper.BindPFlag("chart-use-yfid", chartCmd.Flags().Lookup("use-yfid"))

	chartCmd.Flags().String("events", "div|split|earn", "Events to include (pipe-separated div|split|earn)")
	_ = viper.BindPFlag("chart-events", chartCmd.Flags().Lookup("events"))

	chartCmd.Flags().String("lang", "en-US", "Language for the response")
	_ = viper.BindPFlag("chart-lang", chartCmd.Flags().Lookup("lang"))

	chartCmd.Flags().String("return", "array", "Return style (array|object)")
	_ = viper.BindPFlag("chart-return", chartCmd.Flags().Lookup("return"))

	chartCmd.Flags().StringP("columns", "c", "c", "Table columns (letters; date is always shown): o=open,h=high,l=low,c=close,a=adj close,v=volume,e=event")
	_ = viper.BindPFlag("chart-columns", chartCmd.Flags().Lookup("columns"))

	chartCmd.Flags().Bool("plot", false, "Plot price series next to the table")
	_ = viper.BindPFlag("chart-plot", chartCmd.Flags().Lookup("plot"))
}

func parseChartTimestamp(name, raw string) (*int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		// Interpret millisecond timestamps (commonly used by JS clients).
		if unix > 9_999_999_999 {
			unix = unix / 1000
		}
		return &unix, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			unix := t.Unix()
			return &unix, nil
		}
	}
	return nil, fmt.Errorf("invalid %s: %q (expected unix seconds or parsable date)", name, raw)
}

type chartColumn int

const (
	chartColOpen chartColumn = iota
	chartColHigh
	chartColLow
	chartColClose
	chartColAdjClose
	chartColVolume
	chartColEvent
)

type chartColumns struct {
	order []chartColumn
}

func parseChartColumns(raw string) (chartColumns, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		raw = "ohlcvae"
	}

	out := chartColumns{order: make([]chartColumn, 0, len(raw))}
	seen := map[chartColumn]bool{}

	for _, r := range raw {
		var col chartColumn
		switch r {
		case 'o':
			col = chartColOpen
		case 'h':
			col = chartColHigh
		case 'l':
			col = chartColLow
		case 'c':
			col = chartColClose
		case 'a':
			col = chartColAdjClose
		case 'v':
			col = chartColVolume
		case 'e':
			col = chartColEvent
		default:
			return chartColumns{}, fmt.Errorf("invalid --columns %q: unknown letter %q (allowed: ohlcvae)", raw, string(r))
		}
		if seen[col] {
			continue
		}
		seen[col] = true
		out.order = append(out.order, col)
	}

	if len(out.order) == 0 {
		return chartColumns{}, fmt.Errorf("invalid --columns %q: no columns selected (allowed: ohlcvae)", raw)
	}
	return out, nil
}

func renderChartTable(w *os.File, res yfgo.ChartResult, cols chartColumns, plot bool) {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	applyTableStyle(t)

	extraCols := 2
	if plot {
		extraCols++
	}
	hdr := make(table.Row, 0, 1+len(cols.order)+extraCols)
	hdr = append(hdr, "Date")

	configs := []table.ColumnConfig{}

	for _, c := range cols.order {
		switch c {
		case chartColOpen:
			hdr = append(hdr, "Open")
			configs = append(configs, table.ColumnConfig{Name: "Open", Align: text.AlignRight})
		case chartColHigh:
			hdr = append(hdr, "High")
			configs = append(configs, table.ColumnConfig{Name: "High", Align: text.AlignRight})
		case chartColLow:
			hdr = append(hdr, "Low")
			configs = append(configs, table.ColumnConfig{Name: "Low", Align: text.AlignRight})
		case chartColClose:
			hdr = append(hdr, "Close")
			configs = append(configs, table.ColumnConfig{Name: "Close", Align: text.AlignRight})
		case chartColAdjClose:
			hdr = append(hdr, "Adj Close")
			configs = append(configs, table.ColumnConfig{Name: "Adj Close", Align: text.AlignRight})
		case chartColVolume:
			hdr = append(hdr, "Volume")
			configs = append(configs, table.ColumnConfig{Name: "Volume", Align: text.AlignRight})
		case chartColEvent:
			hdr = append(hdr, "Event")
		}
	}

	hdr = append(hdr, "Δ% -1", "Δ% 1st")
	configs = append(configs,
		table.ColumnConfig{Name: "Δ% -1", Align: text.AlignRight},
		table.ColumnConfig{Name: "Δ% 1st", Align: text.AlignRight},
	)

	if plot {
		hdr = append(hdr, "Plot")
		configs = append(configs, table.ColumnConfig{Name: "Plot", Align: text.AlignLeft})
	}

	t.AppendHeader(hdr)
	if len(configs) > 0 {
		t.SetColumnConfigs(configs)
	}

	priceHint := 2
	if res.Meta.PriceHint != nil {
		priceHint = int(*res.Meta.PriceHint)
		if priceHint < 0 {
			priceHint = 0
		}
		if priceHint > 6 {
			priceHint = 6
		}
	}

	var quoteSeries yfgo.ChartQuoteSeries
	if len(res.Indicators.Quote) > 0 {
		quoteSeries = res.Indicators.Quote[0]
	}
	var adjSeries []*float64
	if len(res.Indicators.AdjClose) > 0 {
		adjSeries = res.Indicators.AdjClose[0].AdjClose
	}

	closeDecimals := priceHint
	if max, ok := chartMaxDecimals(quoteSeries.Close); ok {
		closeDecimals = max
	} else if max, ok := chartMaxDecimalsQuotes(res.Quotes); ok {
		closeDecimals = max
	}

	eventMap := buildChartEvents(res.Events)
	loc := chartLocation(res.Meta)
	timeFormat := "2006-01-02"
	if strings.HasSuffix(res.Meta.DataGranularity, "m") || strings.HasSuffix(res.Meta.DataGranularity, "h") {
		timeFormat = "2006-01-02 15:04"
	}

	rows := make([]chartRow, 0, len(res.Timestamp))
	for idx, ts := range res.Timestamp {
		row := chartRow{
			date:   formatChartTimestamp(ts, loc, timeFormat),
			values: make([]interface{}, 0, len(cols.order)),
		}
		for _, c := range cols.order {
			switch c {
			case chartColOpen:
				row.values = append(row.values, formatChartFloat(floatAt(quoteSeries.Open, idx), priceHint))
			case chartColHigh:
				row.values = append(row.values, formatChartFloat(floatAt(quoteSeries.High, idx), priceHint))
			case chartColLow:
				row.values = append(row.values, formatChartFloat(floatAt(quoteSeries.Low, idx), priceHint))
			case chartColClose:
				row.values = append(row.values, formatChartClose(floatAt(quoteSeries.Close, idx), closeDecimals))
			case chartColAdjClose:
				row.values = append(row.values, formatChartFloat(floatAt(adjSeries, idx), priceHint))
			case chartColVolume:
				row.values = append(row.values, formatChartVolume(intAt(quoteSeries.Volume, idx)))
			case chartColEvent:
				row.values = append(row.values, formatChartEvent(eventMap[ts]))
			}
		}
		row.plotValue = chartPlotValueSeries(quoteSeries, adjSeries, idx)
		rows = append(rows, row)
	}

	// Fallback to quote slice when timestamps missing (rare, e.g. return=array)
	if len(res.Timestamp) == 0 && len(res.Quotes) > 0 {
		for _, q := range res.Quotes {
			row := chartRow{
				date:   formatChartTimestamp(q.Date, loc, timeFormat),
				values: make([]interface{}, 0, len(cols.order)),
			}
			for _, c := range cols.order {
				switch c {
				case chartColOpen:
					row.values = append(row.values, formatChartFloat(q.Open, priceHint))
				case chartColHigh:
					row.values = append(row.values, formatChartFloat(q.High, priceHint))
				case chartColLow:
					row.values = append(row.values, formatChartFloat(q.Low, priceHint))
				case chartColClose:
					row.values = append(row.values, formatChartClose(q.Close, closeDecimals))
				case chartColAdjClose:
					row.values = append(row.values, formatChartFloat(q.AdjClose, priceHint))
				case chartColVolume:
					row.values = append(row.values, formatChartVolume(q.Volume))
				case chartColEvent:
					row.values = append(row.values, formatChartEvent(eventMap[q.Date]))
				}
			}
			row.plotValue = chartPlotValueQuote(q)
			rows = append(rows, row)
		}
	}

	var plotMin, plotMax float64
	plotOK := false
	if plot {
		plotMin, plotMax, plotOK = chartPlotRange(rows)
	}
	firstPlot := chartFirstPlotValue(rows)
	var prevPlot *float64
	for _, row := range rows {
		out := make(table.Row, 0, 1+len(cols.order)+extraCols)
		out = append(out, row.date)
		out = append(out, row.values...)
		chgPrev := formatChartPctChange(row.plotValue, prevPlot)
		chgFirst := formatChartPctChange(row.plotValue, firstPlot)
		out = append(out, chgPrev, chgFirst)
		if plot {
			trend := chartPlotTrendNone
			if row.plotValue != nil && prevPlot != nil {
				switch {
				case *row.plotValue > *prevPlot:
					trend = chartPlotTrendUp
				case *row.plotValue < *prevPlot:
					trend = chartPlotTrendDown
				}
			}
			out = append(out, renderChartPlotCell(row.plotValue, plotMin, plotMax, plotOK, trend))
		}
		if row.plotValue != nil {
			prevPlot = row.plotValue
		}
		t.AppendRow(out)
	}

	t.Render()
}

func chartLocation(meta yfgo.ChartMeta) *time.Location {
	if meta.ExchangeTimezoneName != "" {
		if loc, err := time.LoadLocation(meta.ExchangeTimezoneName); err == nil {
			return loc
		}
	}
	if meta.Timezone != "" && meta.GmtOffset != 0 {
		return time.FixedZone(meta.Timezone, int(meta.GmtOffset))
	}
	return time.UTC
}

func formatChartTimestamp(ts int64, loc *time.Location, layout string) string {
	if ts == 0 {
		return "-"
	}
	return time.Unix(ts, 0).In(loc).Format(layout)
}

func formatChartFloat(val *float64, decimals int) string {
	if val == nil {
		return "-"
	}
	return formatFloatWithCommas(*val, decimals)
}

func formatChartVolume(val *int64) string {
	if val == nil || *val == 0 {
		return "-"
	}
	s := strconv.FormatInt(*val, 10)
	return addCommasIntPart(s)
}

func formatChartEvent(events []string) string {
	if len(events) == 0 {
		return "-"
	}
	return strings.Join(events, "; ")
}

func formatChartClose(val *float64, maxDecimals int) string {
	if val == nil {
		return "-"
	}
	return formatFloatWithCommasTrim(*val, maxDecimals)
}

func formatFloatWithCommasTrim(f float64, maxDecimals int) string {
	if maxDecimals < 0 {
		maxDecimals = 0
	}
	s := formatFloatWithCommas(f, maxDecimals)
	dot := strings.IndexByte(s, '.')
	if dot == -1 {
		return s
	}
	trimmed := strings.TrimRight(s[dot+1:], "0")
	if trimmed == "" {
		return s[:dot]
	}
	return s[:dot+1] + trimmed
}

type chartRow struct {
	date      string
	values    []interface{}
	plotValue *float64
}

const chartPlotWidth = 24

type chartPlotTrend int

const (
	chartPlotTrendNone chartPlotTrend = iota
	chartPlotTrendUp
	chartPlotTrendDown
)

func chartPlotValueSeries(quote yfgo.ChartQuoteSeries, adj []*float64, idx int) *float64 {
	return firstNonNilFloat(
		floatAt(quote.Close, idx),
		floatAt(adj, idx),
		floatAt(quote.Open, idx),
		floatAt(quote.High, idx),
		floatAt(quote.Low, idx),
	)
}

func chartPlotValueQuote(q yfgo.ChartQuote) *float64 {
	return firstNonNilFloat(q.Close, q.AdjClose, q.Open, q.High, q.Low)
}

func firstNonNilFloat(values ...*float64) *float64 {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func chartPlotRange(rows []chartRow) (float64, float64, bool) {
	var min, max float64
	ok := false
	for _, row := range rows {
		if row.plotValue == nil {
			continue
		}
		val := *row.plotValue
		if !ok {
			min, max = val, val
			ok = true
			continue
		}
		if val < min {
			min = val
		}
		if val > max {
			max = val
		}
	}
	return min, max, ok
}

func chartMaxDecimals(values []*float64) (int, bool) {
	maxDecimals := 0
	ok := false
	for _, v := range values {
		if v == nil {
			continue
		}
		decimals := chartDecimalPlaces(*v)
		if decimals > maxDecimals {
			maxDecimals = decimals
		}
		ok = true
	}
	return maxDecimals, ok
}

func chartMaxDecimalsQuotes(quotes []yfgo.ChartQuote) (int, bool) {
	maxDecimals := 0
	ok := false
	for _, q := range quotes {
		if q.Close == nil {
			continue
		}
		decimals := chartDecimalPlaces(*q.Close)
		if decimals > maxDecimals {
			maxDecimals = decimals
		}
		ok = true
	}
	return maxDecimals, ok
}

func chartDecimalPlaces(value float64) int {
	s := strconv.FormatFloat(value, 'f', -1, 64)
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		return len(s) - idx - 1
	}
	return 0
}

func chartFirstPlotValue(rows []chartRow) *float64 {
	for _, row := range rows {
		if row.plotValue != nil {
			return row.plotValue
		}
	}
	return nil
}

func formatChartPctChange(current, base *float64) string {
	if current == nil || base == nil || *base == 0 {
		return "-"
	}
	pct := (*current - *base) / *base * 100
	if math.Abs(pct) < 0.05 {
		pct = 0
	}
	out := fmt.Sprintf("%+.1f%%", pct)
	switch {
	case pct > 0:
		return text.Colors{text.FgGreen}.Sprint(out)
	case pct < 0:
		return text.Colors{text.FgRed}.Sprint(out)
	default:
		return out
	}
}

func renderChartPlotCell(value *float64, min, max float64, ok bool, trend chartPlotTrend) string {
	if !ok || value == nil || chartPlotWidth <= 0 {
		return "-"
	}
	pos := chartPlotPosition(*value, min, max, chartPlotWidth)
	if pos < 0 {
		return "-"
	}
	out := make([]byte, chartPlotWidth)
	for i := range out {
		out[i] = ' '
	}
	out[pos] = '*'
	plot := string(out)
	switch trend {
	case chartPlotTrendUp:
		return text.Colors{text.FgGreen}.Sprint(plot)
	case chartPlotTrendDown:
		return text.Colors{text.FgRed}.Sprint(plot)
	default:
		return plot
	}
}

func chartPlotPosition(value, min, max float64, width int) int {
	if width <= 0 {
		return -1
	}
	if min == max {
		return width / 2
	}
	ratio := (value - min) / (max - min)
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	return int(math.Round(ratio * float64(width-1)))
}

func floatAt(values []*float64, idx int) *float64 {
	if idx >= len(values) || values[idx] == nil {
		return nil
	}
	return values[idx]
}

func intAt(values []*int64, idx int) *int64 {
	if idx >= len(values) || values[idx] == nil {
		return nil
	}
	return values[idx]
}

func buildChartEvents(ev yfgo.ChartEvents) map[int64][]string {
	out := make(map[int64][]string)
	for tsStr, div := range ev.Dividends {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		amount := "-"
		if div.Amount != nil {
			amount = trimZeros(fmt.Sprintf("%.4f", *div.Amount))
		}
		out[ts] = append(out[ts], fmt.Sprintf("dividend %s", amount))
	}
	for tsStr, split := range ev.Splits {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		label := "split"
		switch {
		case split.SplitRatio != "":
			label = fmt.Sprintf("split %s", split.SplitRatio)
		case split.Numerator != nil && split.Denominator != nil && *split.Denominator != 0:
			label = fmt.Sprintf("split %d:%d", *split.Numerator, *split.Denominator)
		}
		out[ts] = append(out[ts], label)
	}
	for tsStr := range ev.Earnings {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		out[ts] = append(out[ts], "earnings")
	}
	return out
}
