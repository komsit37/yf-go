// Package yfapi provides a small Yahoo Finance client focused on the
// quoteSummary endpoint.
//
// Most users should depend on the API interface for easy testing and DI:
//
//  var api yfapi.API = yfapi.NewClient()
//  typed, err := api.QuoteSummaryTyped(ctx, "AAPL", []string{"price", "summaryDetail", "financialData"})
//
// Convenience package-level functions are also available and use a default client:
//
//  raw, err := yfapi.FetchQuoteSummary(ctx, "AAPL", yfapi.DefaultQuoteSummaryModules)
package yfapi

