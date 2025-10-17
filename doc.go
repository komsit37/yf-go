// Package yfgo provides a small Yahoo Finance client for Yahoo Finance APIs,
// including v10 quoteSummary and v7 quote endpoints.
//
// Most users should depend on the API interface for easy testing and DI:
//
//	var api yfgo.API = yfgo.NewClient()
//	typed, err := api.QuoteSummaryTyped(ctx, "AAPL", []yfgo.QuoteSummaryModule{yfgo.ModulePrice, yfgo.ModuleSummaryDetail, yfgo.ModuleFinancialData})
//
// Convenience default API is also available via the interface to avoid
// depending on concrete types:
//
//	raw, err := yfgo.DefaultAPI.QuoteSummary(ctx, "AAPL", yfgo.DefaultQuoteSummaryModules)
//
// The v7 quote endpoint can fetch multiple symbols at once:
//
//	quotes, err := yfgo.DefaultAPI.Quote(ctx, []string{"AAPL", "MSFT"})
//
// Chart data is also supported via the v8/finance/chart endpoint:
//
//	series, err := yfgo.DefaultAPI.Chart(ctx, "AAPL", yfgo.ChartOptions{Range: "5d", Interval: "1h"})
//	typed, err := yfgo.DefaultAPI.ChartTyped(ctx, "AAPL", yfgo.ChartOptions{Range: "5d", Interval: "1h"})
package yfgo
