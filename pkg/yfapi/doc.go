// Package yfapi provides a small Yahoo Finance client focused on the
// quoteSummary endpoint.
//
// Most users should depend on the API interface for easy testing and DI:
//
//	var api yfapi.API = yfapi.NewClient()
//	typed, err := api.QuoteSummaryTyped(ctx, "AAPL", []string{"price", "summaryDetail", "financialData"})
//
// Convenience default API is also available via the interface to avoid
// depending on concrete types:
//
//	raw, err := yfapi.DefaultAPI.QuoteSummary(ctx, "AAPL", yfapi.DefaultQuoteSummaryModules)
package yfapi
