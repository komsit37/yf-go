package cmd

import (
    "context"
    "fmt"
    "os"
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
// When no symbol is provided, it defaults to 3353.T.
var priceCmd = &cobra.Command{
    Use:   "price [symbol]",
    Short: "Get price module for a symbol (shorthand for 'qs -m price')",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        if len(args) != 1 {
            return fmt.Errorf("symbol is required unless --list-modules is used")
        }
        symbol := args[0]

        modules := []yfapi.QuoteSummaryModule{yfapi.ModulePrice}

        ctx := context.Background()
        result, err := yfapi.DefaultAPI.QuoteSummaryTyped(ctx, symbol, modules)
        if err != nil {
            return err
        }
        switch viper.GetString("format") {
        case "json":
            return printJSON(result, viper.GetBool("pretty"))
        case "table":
            full, _ := cmd.Flags().GetBool("full")
            renderPriceTable(os.Stdout, result, full)
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
