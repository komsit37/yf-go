package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"yf/pkg/yfapi"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// priceCmd is a convenience command equivalent to:
//
//	yf qs <symbol> -m price
//
// Supports multiple symbols via comma-separated input or multiple args.
var priceCmd = &cobra.Command{
	Use:   "price <symbol...>",
	Short: "Get price module for one or more symbols (shorthand for 'qs -m price')",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Support comma-separated symbols and multiple args
		symbols := parseSymbols(args)
		if len(symbols) == 0 {
			return fmt.Errorf("symbol is required")
		}

		modules := []yfapi.QuoteSummaryModule{yfapi.ModulePrice}

		// arg parsing done, suppress usage on error after this point
		cmd.SilenceUsage = true
		ctx := context.Background()
		api := strings.ToLower(viper.GetString("api"))
		var results []yfapi.QuoteSummaryTyped
		switch api {
		case "", "quote":
			// Use v7/finance/quote which supports multiple symbols at once
			quotes, err := yfapi.DefaultAPI.Quote(ctx, symbols)
			if err != nil {
				return err
			}
			results = make([]yfapi.QuoteSummaryTyped, 0, len(quotes))
			for _, q := range quotes {
				results = append(results, quoteToSummaryTyped(q))
			}
		case "quotesummary":
			// Fall back to v10/finance/quoteSummary per-symbol
			results = make([]yfapi.QuoteSummaryTyped, 0, len(symbols))
			for _, s := range symbols {
				r, err := yfapi.DefaultAPI.QuoteSummaryTyped(ctx, s, modules)
				if err != nil {
					return err
				}
				results = append(results, r)
			}
		default:
			return fmt.Errorf("unsupported --api: %s (allowed: quote, quotesummary)", api)
		}

		switch viper.GetString("format") {
		case "json":
			if len(results) == 1 {
				if err := printJSON(results[0], viper.GetBool("pretty")); err != nil {
					return err
				}
				return nil
			}
			if err := printJSON(results, viper.GetBool("pretty")); err != nil {
				return err
			}
			return nil
		case "table":
			full, _ := cmd.Flags().GetBool("full")
			renderPriceTableMany(os.Stdout, results, full)
			return nil
		default:
			return fmt.Errorf("unsupported format: %s", viper.GetString("format"))
		}
	},
}

func init() {
	// Set a price-only default format when the user hasn't specified one
	// via CLI flag, env (YF_FORMAT), or config file. This preserves the
	// global default elsewhere but makes `price` default to table output.
	priceCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if !cmd.Flags().Changed("format") && os.Getenv("YF_FORMAT") == "" && !viper.InConfig("format") {
			viper.Set("format", "table")
		}
	}
	// Flag to show full columns (status and time)
	priceCmd.Flags().Bool("full", false, "Show status and time columns")
	// API selection: quote (default) supports multiple symbols, or quotesummary
	priceCmd.Flags().String("api", "quote", "API to use (quote|quotesummary); default quote supports multiple symbols")
	_ = viper.BindPFlag("api", priceCmd.Flags().Lookup("api"))
	rootCmd.AddCommand(priceCmd)
}

// renderPriceTableMany prints a compact table for multiple symbols.
func renderPriceTableMany(w *os.File, results []yfapi.QuoteSummaryTyped, full bool) {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleColoredDark)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateRows = false
	t.Style().Options.SeparateColumns = false

	hdr := table.Row{"Sym", "Price", "Chg", "Chg%", "Prev", "Mkt Cap"}
	if full {
		hdr = append(hdr, "Status", "Time")
	}
	t.AppendHeader(hdr)

	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "Price", Align: text.AlignRight},
		{Name: "Chg", Align: text.AlignRight},
		{Name: "Chg%", Align: text.AlignRight},
		{Name: "Prev", Align: text.AlignRight},
		{Name: "Mkt Cap", Align: text.AlignRight},
	})

	for _, v := range results {
		var (
			symbol       string
			price        string
			prevClose    string
			marketCap    string
			marketStatus string
			dataTimeStr  string
		)
		if v.Price != nil {
			p := v.Price
			symbol = p.Symbol
			price = formatPrice1(p.RegularMarketPrice)
			prevClose = formatPrice1(p.RegularMarketPreviousClose)
			marketCap = formatMarketCap(p.MarketCap)
			marketStatus = p.MarketState
			dataTimeStr = formatMarketTime(p.RegularMarketTime, p.ExchangeTimezoneName)
		}
		chAbs, chPct := changeColumns(v.Price.RegularMarketChange, v.Price.RegularMarketChangePercent)
		row := table.Row{symbol, price, chAbs, chPct, prevClose, marketCap}
		if full {
			row = append(row, marketStatus, dataTimeStr)
		}
		t.AppendRow(row)
	}
	t.Render()
}

// parseSymbols flattens args and comma-separated lists, trimming whitespace.
func parseSymbols(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		parts := strings.Split(a, ",")
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func displayY(n yfapi.YNum) string {
	if n.Fmt != "" {
		return n.Fmt
	}
	if n.Raw != nil {
		return trimZeros(fmtFloat(*n.Raw))
	}
	return "-"
}

func formatChange(ch yfapi.YNum, pct yfapi.YNum) string {
	// Absolute change with thousands separator
	chStr := formatChangeAbs(ch)
	// pct.Fmt usually includes the percent sign; if missing, format.
	pctStr := pct.Fmt
	if pctStr == "" && pct.Raw != nil {
		pctStr = fmt.Sprintf("%.1f%%", *pct.Raw*100)
	}
	// Build base string first
	var base string
	switch {
	case chStr == "-" && pctStr == "":
		base = "-"
	case pctStr == "":
		base = chStr
	default:
		base = fmt.Sprintf("%s (%s)", chStr, pctStr)
	}

	// Colorize based on sign: prefer change.raw, fall back to percent.raw
	var sign float64
	if ch.Raw != nil {
		sign = *ch.Raw
	} else if pct.Raw != nil {
		sign = *pct.Raw
	}
	if sign > 0 {
		return text.Colors{text.FgGreen}.Sprint(base)
	}
	if sign < 0 {
		return text.Colors{text.FgRed}.Sprint(base)
	}
	return base
}

func formatMarketTime(sec int64, tzName string) string {
	if sec == 0 {
		return "-"
	}
	// Convert to local timezone and decide whether to include the date
	tm := time.Unix(sec, 0).In(time.Now().Location())
	now := time.Now()
	sameDay := tm.Year() == now.Year() && tm.YearDay() == now.YearDay()
	if sameDay {
		return tm.Format("15:04 MST")
	}
	return tm.Format("2006-01-02 15:04 MST")
}

// formatPrice1 renders a price with exactly 1 decimal when raw is available.
// Falls back to fmt string or "-" if neither present.
func formatPrice1(n yfapi.YNum) string {
	if n.Raw != nil {
		return formatFloatWithCommas(*n.Raw, 1)
	}
	if n.Fmt != "" {
		// If fmt exists but has more decimals, we won't trim to 1; keep as-is.
		return n.Fmt
	}
	return "-"
}

// formatMarketCap renders a humanized market cap (e.g., 3.83T) when fmt is missing.
// Uses 2 decimals for readability. Falls back to fmt or "-" if no data.
func formatMarketCap(n yfapi.YNum) string {
	if n.Fmt != "" {
		return n.Fmt
	}
	if n.Raw == nil {
		return "-"
	}
	v := *n.Raw
	const (
		K = 1_000
		M = 1_000_000
		B = 1_000_000_000
		T = 1_000_000_000_000
	)
	switch {
	case v >= T:
		return fmt.Sprintf("%.1fT", v/T)
	case v >= B:
		return fmt.Sprintf("%.1fB", v/B)
	case v >= M:
		return fmt.Sprintf("%.1fM", v/M)
	case v >= K:
		return fmt.Sprintf("%.1fK", v/K)
	default:
		return trimZeros(fmt.Sprintf("%.3f", v))
	}
}

// formatChangeAbs renders absolute change with commas and 2 decimals when raw is available.
func formatChangeAbs(n yfapi.YNum) string {
	if n.Raw != nil {
		return formatFloatWithCommas(*n.Raw, 1)
	}
	if n.Fmt != "" {
		return n.Fmt
	}
	return "-"
}

// formatFloatWithCommas formats a float with fixed decimals and thousands separators.
func formatFloatWithCommas(f float64, decimals int) string {
	// Build base with required precision
	fmtStr := fmt.Sprintf("%%.%df", decimals)
	s := fmt.Sprintf(fmtStr, f)
	// Handle sign
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}
	// Split integer and fractional parts
	dot := strings.IndexByte(s, '.')
	intPart, fracPart := s, ""
	if dot >= 0 {
		intPart = s[:dot]
		fracPart = s[dot:]
	}
	return sign + addCommasIntPart(intPart) + fracPart
}

// addCommasIntPart inserts commas into an integer numeric string.
func addCommasIntPart(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}
	// Build from right to left in chunks of 3
	var b strings.Builder
	rem := n % 3
	if rem == 0 {
		rem = 3
	}
	b.WriteString(s[:rem])
	for i := rem; i < n; i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// changeColumns returns separate formatted strings for absolute change and percent change,
// colorized based on sign. Percent uses two decimals.
func changeColumns(ch yfapi.YNum, pct yfapi.YNum) (string, string) {
	abs := formatChangeAbs(ch)
	p := pct.Fmt
	if p == "" && pct.Raw != nil {
		p = fmt.Sprintf("%.1f%%", *pct.Raw*100)
	}
	// Determine sign for coloring
	var sign float64
	if ch.Raw != nil {
		sign = *ch.Raw
	} else if pct.Raw != nil {
		sign = *pct.Raw
	}
	if sign > 0 {
		abs = text.Colors{text.FgGreen}.Sprint(abs)
		p = text.Colors{text.FgGreen}.Sprint(p)
	} else if sign < 0 {
		abs = text.Colors{text.FgRed}.Sprint(abs)
		p = text.Colors{text.FgRed}.Sprint(p)
	}
	if p == "" {
		p = "-"
	}
	return abs, p
}

// quoteToSummaryTyped adapts a v7 Quote into the minimal QuoteSummaryTyped shape
// expected by the price table renderer (i.e., populating only Price module fields used).
func quoteToSummaryTyped(q yfapi.Quote) yfapi.QuoteSummaryTyped {
	// helper to build a YNum from *float64
	ynf := func(p *float64) yfapi.YNum {
		if p == nil {
			return yfapi.YNum{}
		}
		return yfapi.YNum{Raw: p}
	}
	// helper to build a YNum from *int64
	yni := func(p *int64) yfapi.YNum {
		if p == nil {
			return yfapi.YNum{}
		}
		f := float64(*p)
		return yfapi.YNum{Raw: &f}
	}
	// helper for percent values coming from quote API where the numeric
	// value is already a percent (e.g., 0.87 for 0.87%). Convert to fraction.
	ynPercent := func(p *float64) yfapi.YNum {
		if p == nil {
			return yfapi.YNum{}
		}
		f := *p / 100.0
		return yfapi.YNum{Raw: &f}
	}

	pm := &yfapi.PriceModule{
		Symbol:                     q.Symbol,
		ShortName:                  q.ShortName,
		LongName:                   q.LongName,
		Currency:                   q.Currency,
		Exchange:                   q.Exchange,
		FullExchangeName:           q.FullExchangeName,
		MarketState:                q.MarketState,
		RegularMarketChange:        ynf(q.RegularMarketChange),
		RegularMarketPrice:         ynf(q.RegularMarketPrice),
		RegularMarketChangePercent: ynPercent(q.RegularMarketChangePercent),
		RegularMarketTime:          q.RegularMarketTime,
		RegularMarketVolume:        yni(q.RegularMarketVolume),
		AverageDailyVolume3Month:   yni(q.AverageDailyVolume3Month),
		MarketCap:                  yni(q.MarketCap),
		TrailingPE:                 ynf(q.TrailingPE),
		RegularMarketPreviousClose: ynf(q.RegularMarketPreviousClose),
	}
	return yfapi.QuoteSummaryTyped{Price: pm}
}
