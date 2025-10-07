package cmd

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "sort"
    "strings"
    "time"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "yf/pkg/yfapi"
)

var qsCmd = &cobra.Command{
    Use:   "qs [symbol]",
    Short: "Get quote summary for a symbol",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // If listing modules, print and exit early
        if viper.GetBool("list-modules") {
            modules := append([]string(nil), yfapi.AllowedQuoteSummaryModules...)
            sort.Strings(modules)
            var out []byte
            var err error
            if viper.GetBool("pretty") {
                out, err = json.MarshalIndent(modules, "", "  ")
            } else {
                out, err = json.Marshal(modules)
            }
            if err != nil {
                return fmt.Errorf("failed to encode json: %w", err)
            }
            _, _ = os.Stdout.Write(out)
            if viper.GetBool("pretty") {
                _, _ = os.Stdout.WriteString("\n")
            }
            return nil
        }

        if len(args) != 1 {
            return fmt.Errorf("symbol is required unless --list-modules is used")
        }
        symbol := args[0]

        // Modules from flag/env; fallback to defaults
        modules := viper.GetStringSlice("modules")
        if len(modules) == 0 {
            modules = append([]string(nil), yfapi.DefaultQuoteSummaryModules...)
        }
        // Support comma-separated values passed as a single item (e.g., -m a,b,c)
        if len(modules) == 1 && strings.Contains(modules[0], ",") {
            modules = strings.Split(modules[0], ",")
        }
        // Trim whitespace from modules
        for i := range modules {
            modules[i] = strings.TrimSpace(modules[i])
        }
        // Validate requested modules
        if err := validateModules(modules, yfapi.AllowedQuoteSummaryModules); err != nil {
            return err
        }

        // Fetch data
        ctx := context.Background()
        switch viper.GetString("format") {
        case "json":
            result, err := yfapi.FetchQuoteSummary(ctx, symbol, modules)
            if err != nil {
                return err
            }
            var out []byte
            if viper.GetBool("pretty") {
                out, err = json.MarshalIndent(result, "", "  ")
            } else {
                out, err = json.Marshal(result)
            }
            if err != nil {
                return fmt.Errorf("failed to encode json: %w", err)
            }
            _, _ = os.Stdout.Write(out)
            if viper.GetBool("pretty") {
                _, _ = os.Stdout.WriteString("\n")
            }
            return nil
        case "table":
            // For table, convert to typed struct and render a concise summary
            typed, err := yfapi.FetchQuoteSummaryTyped(ctx, symbol, modules)
            if err != nil {
                return err
            }
            renderSummary(os.Stdout, typed)
            return nil
        default:
            return fmt.Errorf("unsupported format: %s", viper.GetString("format"))
        }
    },
}

func init() {
    rootCmd.AddCommand(qsCmd)
    // Add modules flag supporting repeats or comma separated values
    qsCmd.Flags().StringSliceP("modules", "m", yfapi.DefaultQuoteSummaryModules, "QuoteSummary modules (repeat or comma-separated). Use --modules multiple times or a,b,c")
    _ = viper.BindPFlag("modules", qsCmd.Flags().Lookup("modules"))
    // Add list-modules flag to print supported modules and exit without a symbol
    qsCmd.Flags().Bool("list-modules", false, "List supported quoteSummary modules and exit")
    _ = viper.BindPFlag("list-modules", qsCmd.Flags().Lookup("list-modules"))
}

// validateModules ensures all requested modules are supported; returns a helpful error otherwise.
func validateModules(requested, allowed []string) error {
    allowedSet := make(map[string]struct{}, len(allowed))
    for _, m := range allowed {
        allowedSet[m] = struct{}{}
    }
    var invalid []string
    for _, m := range requested {
        if _, ok := allowedSet[m]; !ok {
            invalid = append(invalid, m)
        }
    }
    if len(invalid) == 0 {
        return nil
    }
    sort.Strings(allowed)
    return fmt.Errorf("invalid module(s): %s. Allowed: %s", strings.Join(invalid, ", "), strings.Join(allowed, ", "))
}

// ---- Rendering (table) ----

// Render helpers are CLI concerns; types are from yfapi.
func renderSummary(w *os.File, qs yfapi.QuoteSummaryTyped) {
    p := qs.Price
    sd := qs.SummaryDetail
    f := qs.FinancialData

    // Exchange info header
    fmt.Fprintln(w, "🏦 EXCHANGE INFO")
    writeKV(w, "Symbol", p.Symbol)
    writeKV(w, "Name", coalesce(p.LongName, p.ShortName))
    writeKV(w, "Exchange", joinNonEmpty([]string{p.Exchange, nameOrDash(p.FullExchangeName)}, " "))
    writeKV(w, "Currency", currencyPretty(p.Currency))
    writeKV(w, "Market", upOrDash(p.MarketState))
    tzName := nameOrDash(p.ExchangeTimezoneName)
    if tzName == "-" && p.ExchangeTimezoneShortName != "" {
        tzName = p.ExchangeTimezoneShortName
    }
    writeKV(w, "Last", formatLastUpdate(p.RegularMarketTime, p.ExchangeTimezoneName, p.ExchangeTimezoneShortName))
    fmt.Fprintln(w, sepLine())

    // Price summary
    fmt.Fprintln(w, "📈 PRICE SUMMARY")
    fmt.Fprintln(w, fmt.Sprintf("%-12s %-10s %-10s %-10s",
        "Price", "% Change", "Volume", "Avg Vol (3m)",
    ))
    fmt.Fprintln(w, fmt.Sprintf("%-12s %-10s %-10s %-10s",
        vOrDash(p.RegularMarketPrice),
        withPercentIfMissing(vOrDash(p.RegularMarketChangePercent)),
        prefer(p.RegularMarketVolume, yfapi.YNum{}),
        vOrDash(p.AverageDailyVolume3Month),
    ))
    fmt.Fprintln(w, sepLine())

    // Valuation
    fmt.Fprintln(w, "💹 VALUATION & RANGE (TTM)")
    fmt.Fprintln(w, fmt.Sprintf("%-12s %-10s %-18s",
        "Market Cap", "PE", "52-week Range",
    ))
    fmt.Fprintln(w, fmt.Sprintf("%-12s %-10s %-18s",
        vOrDash(p.MarketCap),
        prefer(p.TrailingPE, sd.TrailingPE),
        joinNonEmpty([]string{vOrDash(sd.FiftyTwoWeekLow), vOrDash(sd.FiftyTwoWeekHigh)}, " - "),
    ))
    fmt.Fprintln(w, sepLine())

    // Dividends
    fmt.Fprintln(w, "🏛 DIVIDENDS")
    fmt.Fprintln(w, fmt.Sprintf("%-10s %-10s %-10s",
        "Yield", "Rate", "Payout",
    ))
    fmt.Fprintln(w, fmt.Sprintf("%-10s %-10s %-10s",
        withPercentIfMissing(vOrDash(sd.DividendYield)),
        vOrDash(sd.DividendRate),
        withPercentIfMissing(vOrDash(sd.PayoutRatio)),
    ))
    fmt.Fprintln(w, sepLine())

    // Profitability
    fmt.Fprintln(w, "📊 PROFITABILITY")
    fmt.Fprintln(w, fmt.Sprintf("%-9s %-9s %-9s %-9s",
        "Gross", "Oper", "EBITDA", "Net",
    ))
    fmt.Fprintln(w, fmt.Sprintf("%-9s %-9s %-9s %-9s",
        withPercentIfMissing(vOrDash(f.GrossMargins)),
        withPercentIfMissing(vOrDash(f.OperatingMargins)),
        withPercentIfMissing(vOrDash(f.EbitdaMargins)),
        withPercentIfMissing(vOrDash(f.ProfitMargins)),
    ))
    fmt.Fprintln(w, sepLine())

    // Returns & Growth
    fmt.Fprintln(w, "🧭 RETURNS & GROWTH")
    fmt.Fprintln(w, fmt.Sprintf("%-10s %-10s", "ROA", "ROE"))
    fmt.Fprintln(w, fmt.Sprintf("%-10s %-10s",
        withPercentIfMissing(vOrDash(f.ReturnOnAssets)),
        withPercentIfMissing(vOrDash(f.ReturnOnEquity)),
    ))

    fmt.Fprintln(w, fmt.Sprintf("%-9s %-8s", "Revenue", "Earnings"))
    fmt.Fprintln(w, fmt.Sprintf("%-9s %-8s",
        vOrDash(f.RevenueGrowth),
        vOrDash(f.EarningsGrowth),
    ))
    fmt.Fprintln(w, sepLine())

    // Liquidity & Leverage
    fmt.Fprintln(w, "🏦 LIQUIDITY & LEVERAGE")
    fmt.Fprintln(w, fmt.Sprintf("%-9s %-7s %-12s", "Current", "Quick", "Debt/Equity"))
    fmt.Fprintln(w, fmt.Sprintf("%-9s %-7s %-12s",
        addSuffix(vOrDash(f.CurrentRatio), "x"),
        addSuffix(vOrDash(f.QuickRatio), "x"),
        withPercentIfMissing(vOrDash(f.DebtToEquity)),
    ))
    fmt.Fprintln(w, sepLine())

    // Cash & Efficiency
    fmt.Fprintln(w, "💵 CASH & EFFICIENCY")
    cur := firstNonEmpty(f.FinancialCurrency, p.Currency)
    fmt.Fprintln(w, fmt.Sprintf("%-16s %-10s %-11s %-11s %-12s",
        "Revenue (TTM)", "EBITDA", "Cash", "Debt", "Cash/Share"))
    fmt.Fprintln(w, fmt.Sprintf("%-16s %-10s %-11s %-11s %-12s",
        valueWithCurrency(f.TotalRevenue, cur),
        valueWithCurrency(f.Ebitda, cur),
        valueWithCurrency(f.TotalCash, cur),
        valueWithCurrency(f.TotalDebt, cur),
        vOrDash(f.CashPerShare),
    ))
}

func writeKV(w *os.File, key, val string) {
    fmt.Fprintf(w, "%-10s %s\n", key, nonEmptyOrDash(val))
}

func coalesce(a, b string) string {
    if a != "" {
        return a
    }
    if b != "" {
        return b
    }
    return "-"
}

func nameOrDash(s string) string {
    if strings.TrimSpace(s) == "" {
        return "-"
    }
    return s
}

func upOrDash(s string) string {
    if strings.TrimSpace(s) == "" {
        return "-"
    }
    return strings.ToUpper(s)
}

func nonEmptyOrDash(s string) string {
    if strings.TrimSpace(s) == "" {
        return "-"
    }
    return s
}

func prefer(a yfapi.YNum, b yfapi.YNum) string {
    va := vOrDash(a)
    if va != "-" {
        return va
    }
    return vOrDash(b)
}

func vOrDash(n yfapi.YNum) string {
    if n.Fmt != "" {
        return n.Fmt
    }
    if n.Raw != nil {
        // Print raw without excessive decimals
        return trimZeros(fmtFloat(*n.Raw))
    }
    return "-"
}

func fmtFloat(f float64) string {
    // choose a compact format with up to 3 decimals
    s := fmt.Sprintf("%.3f", f)
    return s
}

func trimZeros(s string) string {
    s = strings.TrimRight(s, "0")
    s = strings.TrimRight(s, ".")
    if s == "" {
        return "0"
    }
    return s
}

func labeled(label, val string) string {
    if val == "-" {
        return fmt.Sprintf("%s %s", label, val)
    }
    return fmt.Sprintf("%s %s", label, val)
}

func labeledWithUnit(label, val string) string { return labeled(label, val) }

func addSuffix(val, suffix string) string {
    if val == "-" {
        return val
    }
    // Avoid double-suffixing when fmt already contains unit
    if strings.HasSuffix(val, suffix) {
        return val
    }
    return val + suffix
}

func withPercentIfMissing(val string) string {
    if val == "-" || strings.Contains(val, "%") {
        return val
    }
    return val + "%"
}

func valueWithCurrency(n yfapi.YNum, currency string) string {
    if n.Fmt != "" {
        return fmt.Sprintf("%s %s", n.Fmt, currency)
    }
    if n.Raw != nil {
        return fmt.Sprintf("%s %s", trimZeros(fmtFloat(*n.Raw)), currency)
    }
    return "-"
}

func firstNonEmpty(a, b string) string {
    if strings.TrimSpace(a) != "" {
        return a
    }
    return b
}

func formatLastUpdate(unix int64, tzName, tzShort string) string {
    if unix == 0 {
        return "-"
    }
    t := time.Unix(unix, 0).UTC()
    if tzName != "" {
        if loc, err := time.LoadLocation(tzName); err == nil {
            t = t.In(loc)
            return t.Format("2006-01-02 15:04 MST")
        }
    }
    // Fallback to short name if provided (without offset conversion)
    if tzShort != "" {
        return t.Format("2006-01-02 15:04 ") + tzShort
    }
    return t.Format("2006-01-02 15:04 UTC")
}

func currencyPretty(code string) string {
    sym := currencySymbol(code)
    if sym == "" {
        return nonEmptyOrDash(code)
    }
    return fmt.Sprintf("%s (%s)", code, sym)
}

func currencySymbol(code string) string {
    switch strings.ToUpper(code) {
    case "USD":
        return "$"
    case "JPY":
        return "￥"
    case "EUR":
        return "€"
    case "GBP":
        return "£"
    case "CNY", "RMB":
        return "¥"
    case "HKD":
        return "HK$"
    case "CAD":
        return "C$"
    case "AUD":
        return "A$"
    case "CHF":
        return "Fr"
    case "SEK", "NOK", "DKK":
        return "kr"
    case "INR":
        return "₹"
    case "KRW":
        return "₩"
    case "SGD":
        return "S$"
    case "NZD":
        return "NZ$"
    case "ZAR":
        return "R"
    case "RUB":
        return "₽"
    case "TRY":
        return "₺"
    case "BRL":
        return "R$"
    case "MXN":
        return "$"
    default:
        return ""
    }
}

func sepLine() string { return strings.Repeat("─", 62) }

func joinNonEmpty(items []string, sep string) string {
    out := make([]string, 0, len(items))
    for _, s := range items {
        s = strings.TrimSpace(s)
        if s != "" && s != "-" {
            out = append(out, s)
        }
    }
    if len(out) == 0 {
        return "-"
    }
    return strings.Join(out, sep)
}
