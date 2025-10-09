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

// renderPriceTable prints a compact human-readable table for the price module.
// Columns: symbol, price, change (change%), prev close, market cap, market status, data source time
func renderPriceTable(w *os.File, v yfapi.QuoteSummaryTyped, full bool) {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	t.Style().Options.DrawBorder = true
	t.Style().Options.SeparateRows = false
	t.Style().Options.SeparateColumns = true

	hdr := table.Row{"Sym", "Price", "Chg (Chg%)", "Prev", "Mkt Cap"}
	if full {
		hdr = append(hdr, "Status", "Time")
	}
	t.AppendHeader(hdr)

	var (
		symbol       string
		price        string
		change       string
		prevClose    string
		marketCap    string
		marketStatus string
		dataTimeStr  string
	)

	if v.Price != nil {
		p := v.Price
		symbol = p.Symbol
		price = displayY(p.RegularMarketPrice)
		change = formatChange(p.RegularMarketChange, p.RegularMarketChangePercent)
		prevClose = displayY(p.RegularMarketPreviousClose)
		marketCap = displayY(p.MarketCap)
		marketStatus = p.MarketState
		dataTimeStr = formatMarketTime(p.RegularMarketTime, p.ExchangeTimezoneName)
	}

	// Align numeric-ish columns to the right
	cfgs := []table.ColumnConfig{
		{Name: "Price", Align: text.AlignRight},
		{Name: "Chg (Chg%)", Align: text.AlignRight},
		{Name: "Prev", Align: text.AlignRight},
		{Name: "Mkt Cap", Align: text.AlignRight},
	}
	t.SetColumnConfigs(cfgs)

	row := table.Row{symbol, price, change, prevClose, marketCap}
	if full {
		row = append(row, marketStatus, dataTimeStr)
	}
	t.AppendRow(row)
	t.Render()
}

// renderPriceTableMany prints a compact table for multiple symbols.
func renderPriceTableMany(w *os.File, results []yfapi.QuoteSummaryTyped, full bool) {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	t.Style().Options.DrawBorder = true
	t.Style().Options.SeparateRows = false
	t.Style().Options.SeparateColumns = true

	hdr := table.Row{"Sym", "Price", "Chg (Chg%)", "Prev", "Mkt Cap"}
	if full {
		hdr = append(hdr, "Status", "Time")
	}
	t.AppendHeader(hdr)

	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "Price", Align: text.AlignRight},
		{Name: "Chg (Chg%)", Align: text.AlignRight},
		{Name: "Prev", Align: text.AlignRight},
		{Name: "Mkt Cap", Align: text.AlignRight},
	})

	for _, v := range results {
		var (
			symbol       string
			price        string
			change       string
			prevClose    string
			marketCap    string
			marketStatus string
			dataTimeStr  string
		)
		if v.Price != nil {
			p := v.Price
			symbol = p.Symbol
			price = displayY(p.RegularMarketPrice)
			change = formatChange(p.RegularMarketChange, p.RegularMarketChangePercent)
			prevClose = displayY(p.RegularMarketPreviousClose)
			marketCap = displayY(p.MarketCap)
			marketStatus = p.MarketState
			dataTimeStr = formatMarketTime(p.RegularMarketTime, p.ExchangeTimezoneName)
		}
		row := table.Row{symbol, price, change, prevClose, marketCap}
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
	chStr := displayY(ch)
	// pct.Fmt usually includes the percent sign; if missing, format.
	pctStr := pct.Fmt
	if pctStr == "" && pct.Raw != nil {
		pctStr = trimZeros(fmtFloat(*pct.Raw*100)) + "%"
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
