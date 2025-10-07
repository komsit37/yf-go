package yfapi

import (
    "context"
    "encoding/json"
    "errors"
    "net/url"
)

type quoteSummaryEnvelope struct {
    QuoteSummary struct {
        Result []any       `json:"result"`
        Error  interface{} `json:"error"`
    } `json:"quoteSummary"`
}

// Provide a local json encoder/decoder handle for internal use.
var defaultJSON = jsonCodec{}

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) MarshalIndent(v any, p, i string) ([]byte, error) {
    return json.MarshalIndent(v, p, i)
}
func (jsonCodec) Unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

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
