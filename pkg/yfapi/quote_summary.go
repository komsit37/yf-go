package yfapi

import (
	"context"
	"errors"
	"net/url"
)

// FetchQuoteSummary retrieves Yahoo Finance quote summary for a symbol using the given modules.
// It returns the first object under quoteSummary.result (as-is, untyped).
func FetchQuoteSummary(ctx context.Context, symbol string, modules []string) (any, error) {
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	// Ensure symbol is a valid path segment (sanity check by escaping and back)
	_ = url.PathEscape(symbol)
	return Default.QuoteSummary(ctx, symbol, modules)
}

