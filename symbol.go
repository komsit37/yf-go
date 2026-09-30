package yfgo

import "strings"

// NormalizeSymbol converts canonical market IDs to Yahoo Finance symbols.
// Unrecognized prefixes and symbols already in Yahoo's format pass through.
func NormalizeSymbol(symbol string) string {
	prefix, ticker, ok := strings.Cut(symbol, ":")
	if !ok || ticker == "" {
		return symbol
	}
	switch strings.ToUpper(prefix) {
	case "JP":
		return ticker + ".T"
	case "US":
		return ticker
	default:
		return symbol
	}
}
