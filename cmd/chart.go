package cmd

import (
	"fmt"
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

		period1, err := parseChartTimestamp("period1", viper.GetString("chart-period1"))
		if err != nil {
			return err
		}
		period2, err := parseChartTimestamp("period2", viper.GetString("chart-period2"))
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
			renderChartTable(os.Stdout, typed, cols)
			return nil
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}
	},
}

func init() {
	rootCmd.AddCommand(chartCmd)

	chartCmd.Flags().String("interval", "1mo", "Data interval (1m, 2m, 5m, 15m, 30m, 60m, 90m, 1h, 1d, 5d, 1wk, 1mo, 3mo)")
	_ = viper.BindPFlag("chart-interval", chartCmd.Flags().Lookup("interval"))

	chartCmd.Flags().String("range", "1y", "Date range (e.g. 1d, 5d, 1mo, 3mo, 6mo, 1y, 5y, ytd, max). Use period flags for a custom window.")
	_ = viper.BindPFlag("chart-range", chartCmd.Flags().Lookup("range"))

	chartCmd.Flags().String("period1", "", "Custom range start (unix seconds or date: YYYY-MM-DD, RFC3339).")
	_ = viper.BindPFlag("chart-period1", chartCmd.Flags().Lookup("period1"))

	chartCmd.Flags().String("period2", "", "Custom range end (unix seconds or date: YYYY-MM-DD, RFC3339). Defaults to now when omitted.")
	_ = viper.BindPFlag("chart-period2", chartCmd.Flags().Lookup("period2"))

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

func renderChartTable(w *os.File, res yfgo.ChartResult, cols chartColumns) {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	applyTableStyle(t)

	hdr := make(table.Row, 0, 1+len(cols.order))
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

	eventMap := buildChartEvents(res.Events)
	loc := chartLocation(res.Meta)
	timeFormat := "2006-01-02"
	if strings.HasSuffix(res.Meta.DataGranularity, "m") || strings.HasSuffix(res.Meta.DataGranularity, "h") {
		timeFormat = "2006-01-02 15:04"
	}

	for idx, ts := range res.Timestamp {
		row := make(table.Row, 0, 1+len(cols.order))
		row = append(row, formatChartTimestamp(ts, loc, timeFormat))
		for _, c := range cols.order {
			switch c {
			case chartColOpen:
				row = append(row, formatChartFloat(floatAt(quoteSeries.Open, idx), priceHint))
			case chartColHigh:
				row = append(row, formatChartFloat(floatAt(quoteSeries.High, idx), priceHint))
			case chartColLow:
				row = append(row, formatChartFloat(floatAt(quoteSeries.Low, idx), priceHint))
			case chartColClose:
				row = append(row, formatChartFloat(floatAt(quoteSeries.Close, idx), priceHint))
			case chartColAdjClose:
				row = append(row, formatChartFloat(floatAt(adjSeries, idx), priceHint))
			case chartColVolume:
				row = append(row, formatChartVolume(intAt(quoteSeries.Volume, idx)))
			case chartColEvent:
				row = append(row, formatChartEvent(eventMap[ts]))
			}
		}
		t.AppendRow(row)
	}

	// Fallback to quote slice when timestamps missing (rare, e.g. return=array)
	if len(res.Timestamp) == 0 && len(res.Quotes) > 0 {
		for _, q := range res.Quotes {
			row := make(table.Row, 0, 1+len(cols.order))
			row = append(row, formatChartTimestamp(q.Date, loc, timeFormat))
			for _, c := range cols.order {
				switch c {
				case chartColOpen:
					row = append(row, formatChartFloat(q.Open, priceHint))
				case chartColHigh:
					row = append(row, formatChartFloat(q.High, priceHint))
				case chartColLow:
					row = append(row, formatChartFloat(q.Low, priceHint))
				case chartColClose:
					row = append(row, formatChartFloat(q.Close, priceHint))
				case chartColAdjClose:
					row = append(row, formatChartFloat(q.AdjClose, priceHint))
				case chartColVolume:
					row = append(row, formatChartVolume(q.Volume))
				case chartColEvent:
					row = append(row, formatChartEvent(eventMap[q.Date]))
				}
			}
			t.AppendRow(row)
		}
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
