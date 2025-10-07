package cmd

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/spf13/cobra"
    "yf/internal/yfapi"
)

// yNum mirrors Yahoo's quoteSummary number objects which commonly contain
// raw/fmt/longFmt. We primarily prefer the human-friendly Fmt when available.
type yNum struct {
    Raw     *float64 `json:"raw"`
    Fmt     string   `json:"fmt"`
    LongFmt string   `json:"longFmt"`
}

type priceModule struct {
    Symbol                        string `json:"symbol"`
    ShortName                     string `json:"shortName"`
    LongName                      string `json:"longName"`
    Currency                      string `json:"currency"`
    Exchange                      string `json:"exchange"`
    FullExchangeName              string `json:"fullExchangeName"`
    MarketState                   string `json:"marketState"`
    ExchangeTimezoneName          string `json:"exchangeTimezoneName"`
    ExchangeTimezoneShortName     string `json:"exchangeTimezoneShortName"`
    RegularMarketPrice            yNum   `json:"regularMarketPrice"`
    RegularMarketChangePercent    yNum   `json:"regularMarketChangePercent"`
    RegularMarketTime             int64  `json:"regularMarketTime"`
    RegularMarketVolume           yNum   `json:"regularMarketVolume"`
    AverageDailyVolume3Month      yNum   `json:"averageDailyVolume3Month"`
    MarketCap                     yNum   `json:"marketCap"`
    TrailingPE                    yNum   `json:"trailingPE"`
}

type summaryDetailModule struct {
    DividendYield     yNum `json:"dividendYield"`
    DividendRate      yNum `json:"dividendRate"`
    PayoutRatio       yNum `json:"payoutRatio"`
    FiftyTwoWeekLow   yNum `json:"fiftyTwoWeekLow"`
    FiftyTwoWeekHigh  yNum `json:"fiftyTwoWeekHigh"`
    TrailingPE        yNum `json:"trailingPE"`
}

type financialDataModule struct {
    FinancialCurrency string `json:"financialCurrency"`

    GrossMargins     yNum `json:"grossMargins"`
    OperatingMargins yNum `json:"operatingMargins"`
    EbitdaMargins    yNum `json:"ebitdaMargins"`
    ProfitMargins    yNum `json:"profitMargins"`

    ReturnOnAssets yNum `json:"returnOnAssets"`
    ReturnOnEquity yNum `json:"returnOnEquity"`

    RevenueGrowth  yNum `json:"revenueGrowth"`
    EarningsGrowth yNum `json:"earningsGrowth"`

    CurrentRatio yNum `json:"currentRatio"`
    QuickRatio   yNum `json:"quickRatio"`
    DebtToEquity yNum `json:"debtToEquity"`

    TotalRevenue yNum `json:"totalRevenue"`
    Ebitda       yNum `json:"ebitda"`
    TotalCash    yNum `json:"totalCash"`
    TotalDebt    yNum `json:"totalDebt"`
    CashPerShare yNum `json:"totalCashPerShare"`
}

type quoteSummaryResult struct {
    Price         priceModule         `json:"price"`
    SummaryDetail summaryDetailModule `json:"summaryDetail"`
    FinancialData financialDataModule `json:"financialData"`
}

var sumCmd = &cobra.Command{
    Use:   "sum [symbol]",
    Short: "Show a formatted summary from quote summary",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        symbol := args[0]
        ctx := context.Background()

        // Reuse the same modules as qs by default.
        result, err := yfapi.FetchQuoteSummary(ctx, symbol, defaultModules)
        if err != nil {
            return err
        }

        // Convert untyped into our typed struct
        var qs quoteSummaryResult
        b, _ := json.Marshal(result)
        if err := json.Unmarshal(b, &qs); err != nil {
            return fmt.Errorf("failed to decode quote summary: %w", err)
        }

        renderSummary(os.Stdout, qs)
        return nil
    },
}

func init() {
    rootCmd.AddCommand(sumCmd)
}

// render helpers

func renderSummary(w *os.File, qs quoteSummaryResult) {
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
    writeKV(w, "Time Zone", tzName)
    writeKV(w, "Last Update", formatLastUpdate(p.RegularMarketTime, p.ExchangeTimezoneName, p.ExchangeTimezoneShortName))
    fmt.Fprintln(w, sepLine())

    // Price & Performance
    fmt.Fprintln(w, "📈 PRICE & PERFORMANCE")
    fmt.Fprintln(w, fmt.Sprintf("%-8s %-7s %-6s %-7s %-7s %-6s %-7s",
        "Price", "Chg%", "PE", "MCap", "DivYld", "Div", "Payout"))
    fmt.Fprintln(w, fmt.Sprintf("%-8s %-7s %-6s %-7s %-7s %-6s %-7s",
        vOrDash(p.RegularMarketPrice),
        vOrDash(p.RegularMarketChangePercent),
        prefer(sd.TrailingPE, p.TrailingPE),
        vOrDash(p.MarketCap),
        vOrDash(sd.DividendYield),
        vOrDash(sd.DividendRate),
        withPercentIfMissing(vOrDash(sd.PayoutRatio)),
    ))
    fmt.Fprintln(w)
    fmt.Fprintln(w, fmt.Sprintf("%-12s %-8s %-12s",
        "52W Range", "Volume", "AvgVol(3M)"))
    fmt.Fprintln(w, fmt.Sprintf("%-12s %-8s %-12s",
        fmt.Sprintf("%s–%s", vOrDash(sd.FiftyTwoWeekLow), vOrDash(sd.FiftyTwoWeekHigh)),
        vOrDash(p.RegularMarketVolume),
        vOrDash(p.AverageDailyVolume3Month),
    ))
    fmt.Fprintln(w, sepLine())

    // Profitability
    fmt.Fprintln(w, "💰 PROFITABILITY")
    fmt.Fprintln(w, fmt.Sprintf("%-7s %-6s %-8s %-7s %-5s %-5s",
        "Gross", "Oper", "EBITDA", "Net", "ROA", "ROE"))
    fmt.Fprintln(w, fmt.Sprintf("%-7s %-6s %-8s %-7s %-5s %-5s",
        vOrDash(f.GrossMargins),
        vOrDash(f.OperatingMargins),
        vOrDash(f.EbitdaMargins),
        vOrDash(f.ProfitMargins),
        vOrDash(f.ReturnOnAssets),
        vOrDash(f.ReturnOnEquity),
    ))
    fmt.Fprintln(w, sepLine())

    // Growth
    fmt.Fprintln(w, "🚀 GROWTH")
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

func prefer(a yNum, b yNum) string {
    va := vOrDash(a)
    if va != "-" {
        return va
    }
    return vOrDash(b)
}

func vOrDash(n yNum) string {
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

func valueWithCurrency(n yNum, currency string) string {
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
