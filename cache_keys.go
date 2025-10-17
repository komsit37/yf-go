package yfgo

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func cacheKeyQuoteSummary(symbol string, modules []QuoteSummaryModule) string {
	mods := ModulesToStrings(modules)
	sort.Strings(mods)
	return fmt.Sprintf("quotesummary:%s:%s", strings.ToUpper(symbol), strings.Join(mods, ","))
}

func cacheKeyQuote(symbols []string) string {
	if len(symbols) == 0 {
		return "quote:"
	}
	copySymbols := append([]string(nil), symbols...)
	for i := range copySymbols {
		copySymbols[i] = strings.ToUpper(strings.TrimSpace(copySymbols[i]))
	}
	sort.Strings(copySymbols)
	return fmt.Sprintf("quote:%s", strings.Join(copySymbols, ","))
}

func cacheKeyChart(symbol string, opts ChartOptions) string {
	query := buildChartQuery(opts)
	return fmt.Sprintf("chart:%s:%s", strings.ToUpper(symbol), query.Encode())
}

func buildChartQuery(opts ChartOptions) url.Values {
	q := url.Values{}
	for k, vals := range opts.AdditionalQuery {
		for _, v := range vals {
			q.Add(k, v)
		}
	}

	if opts.Interval != "" {
		q.Set("interval", opts.Interval)
	}
	if opts.Range != "" {
		q.Set("range", opts.Range)
	}
	if opts.Period1 != nil {
		q.Set("period1", fmt.Sprintf("%d", *opts.Period1))
	}
	if opts.Period2 != nil {
		q.Set("period2", fmt.Sprintf("%d", *opts.Period2))
	}
	if opts.IncludePrePost != nil {
		q.Set("includePrePost", fmt.Sprintf("%t", *opts.IncludePrePost))
	}
	if opts.Events != "" {
		q.Set("events", opts.Events)
	}
	if opts.Lang != "" {
		q.Set("lang", opts.Lang)
	}
	if opts.UseYfid != nil {
		q.Set("useYfid", fmt.Sprintf("%t", *opts.UseYfid))
	}
	if opts.ReturnType != "" {
		q.Set("return", opts.ReturnType)
	}

	if _, ok := q["interval"]; !ok {
		q.Set("interval", "1d")
	}
	if _, ok := q["events"]; !ok {
		q.Set("events", "div|split|earn")
	}
	if _, ok := q["lang"]; !ok {
		q.Set("lang", "en-US")
	}
	if _, ok := q["return"]; !ok {
		q.Set("return", "array")
	}
	if _, ok := q["includePrePost"]; !ok {
		q.Set("includePrePost", "true")
	}
	if _, ok := q["useYfid"]; !ok {
		q.Set("useYfid", "true")
	}
	if _, hasRange := q["range"]; !hasRange && opts.Period1 == nil {
		q.Set("range", "1mo")
	}
	return q
}
