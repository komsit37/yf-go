package yfgo

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SheetRow holds one formatted financial metric or company detail.
type SheetRow struct {
	Label string
	Value string
}

// SheetData contains the fundamentals displayed below a sheet's chart.
type SheetData struct {
	Title   string
	Metrics []SheetRow
	Company []SheetRow
	Summary string
	Website string
}

var sheetModules = []QuoteSummaryModule{
	ModuleAssetProfile, ModuleSummaryDetail, ModuleDefaultKeyStatistics,
	ModuleFinancialData, ModulePrice, ModuleCalendarEvents,
}

// ParseSheetData formats a QuoteSummary result, omitting missing metrics.
// Yahoo raw numbers take precedence over its sometimes inconsistent fmt strings.
func ParseSheetData(summary any) (SheetData, error) {
	b, err := json.Marshal(summary)
	if err != nil {
		return SheetData{}, err
	}
	var root map[string]map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		return SheetData{}, fmt.Errorf("invalid sheet fundamentals: %w", err)
	}
	profile, detail, stats := root["assetProfile"], root["summaryDetail"], root["defaultKeyStatistics"]
	financial, price, calendar := root["financialData"], root["price"], root["calendarEvents"]
	str := func(module map[string]any, key string) string {
		value, _ := module[key].(string)
		return strings.TrimSpace(value)
	}
	currency := str(price, "currency")
	if currency == "" {
		currency = str(detail, "currency")
	}
	financialCurrency := str(financial, "financialCurrency")
	if financialCurrency == "" {
		financialCurrency = currency
	}
	out := SheetData{Title: str(price, "longName"), Summary: str(profile, "longBusinessSummary"), Website: sheetWebsite(str(profile, "website"))}
	if out.Title == "" {
		out.Title = str(price, "shortName")
	}
	add := func(label, value string) {
		if value != "" {
			out.Metrics = append(out.Metrics, SheetRow{label, value})
		}
	}
	value := func(module map[string]any, key, kind, unit string) string {
		return sheetFinancial(module[key], kind, unit)
	}
	first := func(a, b string) string {
		if a != "" {
			return a
		}
		return b
	}
	add("Mkt cap", first(value(price, "marketCap", "money", currency), value(detail, "marketCap", "money", currency)))
	add("P/E", value(detail, "trailingPE", "ratio", ""))
	add("Fwd P/E", first(value(detail, "forwardPE", "ratio", ""), value(stats, "forwardPE", "ratio", "")))
	add("P/B", value(stats, "priceToBook", "ratio", ""))
	add("EV/EBITDA", value(stats, "enterpriseToEbitda", "ratio", ""))
	add("Div yld", value(detail, "dividendYield", "percent", ""))
	add("Payout", value(detail, "payoutRatio", "percent", ""))
	add("ROE", value(financial, "returnOnEquity", "percent", ""))
	add("ROA", value(financial, "returnOnAssets", "percent", ""))
	add("Op mgn", value(financial, "operatingMargins", "percent", ""))
	add("Net mgn", value(financial, "profitMargins", "percent", ""))
	add("Rev grow", value(financial, "revenueGrowth", "change", ""))
	add("Earn grow", value(financial, "earningsGrowth", "change", ""))
	add("D/E", value(financial, "debtToEquity", "debtPercent", ""))
	add("Current", value(financial, "currentRatio", "ratio", ""))
	add("Cash", value(financial, "totalCash", "money", financialCurrency))
	add("Debt", value(financial, "totalDebt", "money", financialCurrency))
	add("52w low", value(detail, "fiftyTwoWeekLow", "price", currency))
	add("52w high", value(detail, "fiftyTwoWeekHigh", "price", currency))
	low, lowOK := sheetRaw(detail["fiftyTwoWeekLow"])
	high, highOK := sheetRaw(detail["fiftyTwoWeekHigh"])
	last, lastOK := sheetRaw(price["regularMarketPrice"])
	if lowOK && highOK && lastOK && high > low {
		add("52w pos", sheetFinancial((last-low)/(high-low), "percent", ""))
	}
	add("Vol 3M", value(price, "averageDailyVolume3Month", "compact", ""))
	if earnings, ok := calendar["earnings"].(map[string]any); ok {
		if dates, ok := earnings["earningsDate"].([]any); ok && len(dates) > 0 {
			add("Earnings", sheetFinancial(dates[0], "date", ""))
		}
	}
	add("Ex-div", value(detail, "exDividendDate", "date", ""))
	for _, row := range []SheetRow{
		{"Sector", str(profile, "sector")}, {"Industry", str(profile, "industry")},
		{"Country", str(profile, "country")}, {"Exchange", str(price, "exchangeName")},
		{"Employees", value(profile, "fullTimeEmployees", "integer", "")},
		{"Website", strings.TrimPrefix(strings.TrimPrefix(str(profile, "website"), "https://"), "http://")},
	} {
		if row.Value != "" {
			out.Company = append(out.Company, row)
		}
	}
	return out, nil
}

func sheetWebsite(raw string) string {
	website, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || website.Hostname() == "" || (website.Scheme != "http" && website.Scheme != "https") {
		return ""
	}
	return website.String()
}

func sheetRaw(value any) (float64, bool) {
	if field, ok := value.(map[string]any); ok {
		value = field["raw"]
	}
	number, ok := value.(float64)
	return number, ok && !math.IsNaN(number) && !math.IsInf(number, 0)
}

func sheetFinancial(field any, kind, currency string) string {
	value, ok := sheetRaw(field)
	if !ok {
		if object, ok := field.(map[string]any); ok {
			if formatted, ok := object["fmt"].(string); ok {
				formatted = strings.TrimSpace(formatted)
				switch strings.ToLower(formatted) {
				case "", "-", "n/a", "null", "undefined", "nan":
					return ""
				}
				return formatted
			}
		}
		return ""
	}
	sign := ""
	if value < 0 {
		sign, value = "−", -value
	}
	symbol := map[string]string{"USD": "$", "JPY": "¥", "EUR": "€", "GBP": "£", "CNY": "CN¥", "HKD": "HK$", "CAD": "CA$", "AUD": "A$"}[strings.ToUpper(currency)]
	if symbol == "" && currency != "" {
		symbol = strings.ToUpper(currency) + " "
	}
	switch kind {
	case "compact", "money":
		unit := ""
		for _, scale := range []struct {
			amount float64
			suffix string
		}{{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "K"}} {
			if value >= scale.amount {
				value, unit = value/scale.amount, scale.suffix
				break
			}
		}
		decimals := 2
		if value >= 100 {
			decimals = 0
		} else if value >= 10 {
			decimals = 1
		}
		number := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', decimals, 64), "0"), ".")
		if decimals == 0 {
			number = strconv.FormatFloat(value, 'f', 0, 64)
		}
		if kind == "compact" {
			symbol = ""
		}
		return sign + symbol + number + unit
	case "price":
		number := sheetGrouped(value, 2)
		if strings.EqualFold(currency, "JPY") {
			number = strings.TrimRight(strings.TrimRight(number, "0"), ".")
		}
		return sign + symbol + number
	case "ratio":
		return sign + fmt.Sprintf("%.2f×", value)
	case "percent", "change", "debtPercent":
		if kind != "debtPercent" {
			value *= 100
		}
		if math.Round(value*10) == 0 {
			sign = ""
		}
		if kind == "change" && sign == "" && math.Round(value*10) != 0 {
			sign = "+"
		}
		return sign + fmt.Sprintf("%.1f%%", value)
	case "integer":
		return sign + sheetGrouped(value, 0)
	case "date":
		if sign != "" {
			value = -value
		}
		return time.Unix(int64(value), 0).UTC().Format("2006-01-02")
	}
	return ""
}

func sheetGrouped(value float64, decimals int) string {
	parts := strings.SplitN(strconv.FormatFloat(value, 'f', decimals, 64), ".", 2)
	integer := parts[0]
	for index := len(integer) - 3; index > 0; index -= 3 {
		integer = integer[:index] + "," + integer[index:]
	}
	if len(parts) == 2 {
		integer += "." + parts[1]
	}
	return integer
}
