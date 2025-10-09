// Package yfapi provides a small Yahoo Finance client for Yahoo Finance APIs,
// including v10 quoteSummary and v7 quote endpoints.
//
// Most users should depend on the API interface for easy testing and DI:
//
//	var api yfapi.API = yfapi.NewClient()
//	typed, err := api.QuoteSummaryTyped(ctx, "AAPL", []yfapi.QuoteSummaryModule{yfapi.ModulePrice, yfapi.ModuleSummaryDetail, yfapi.ModuleFinancialData})
//
// Convenience default API is also available via the interface to avoid
// depending on concrete types:
//
//	raw, err := yfapi.DefaultAPI.QuoteSummary(ctx, "AAPL", yfapi.DefaultQuoteSummaryModules)
//
// The v7 quote endpoint can fetch multiple symbols at once:
//
//	quotes, err := yfapi.DefaultAPI.Quote(ctx, []string{"AAPL", "MSFT"})
package yfapi
