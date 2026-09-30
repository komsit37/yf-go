package yfgo

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeSymbol(t *testing.T) {
	tests := []struct{ input, want string }{
		{"JP:7203", "7203.T"}, {"jp:7203", "7203.T"}, {"Jp:9984", "9984.T"},
		{"US:AAPL", "AAPL"}, {"us:BRK-B", "BRK-B"}, {"Us:aapl", "aapl"},
		{"7203.T", "7203.T"}, {"^N225", "^N225"}, {"BRK-B", "BRK-B"},
		{"EU:SAP", "EU:SAP"}, {"JP:", "JP:"}, {"US:", "US:"}, {"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeSymbol(tt.input); got != tt.want {
				t.Fatalf("NormalizeSymbol(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestClientNormalizesSymbolsAndCacheKeys(t *testing.T) {
	t.Setenv("YF_HOME", t.TempDir())
	for _, endpoint := range []string{"chart", "quote", "qs"} {
		for _, tt := range []struct{ input, want string }{{"jp:7203", "7203.T"}, {"US:AAPL", "AAPL"}} {
			t.Run(endpoint+"/"+tt.input, func(t *testing.T) {
				client := NewClient()
				client.sessionWarmed, client.crumb = true, "crumb"
				calls := 0
				client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					body := ""
					switch endpoint {
					case "chart":
						if !strings.HasSuffix(req.URL.Path, "/"+tt.want) {
							t.Fatalf("unexpected chart path %s", req.URL.Path)
						}
						body = `{"chart":{"result":[{"meta":{}}],"error":null}}`
					case "quote":
						if got := req.URL.Query().Get("symbols"); got != tt.want {
							t.Fatalf("symbols = %q, want %q", got, tt.want)
						}
						body = `{"quoteResponse":{"result":[{"symbol":"` + tt.want + `"}],"error":null}}`
					case "qs":
						if !strings.HasSuffix(req.URL.Path, "/"+tt.want) {
							t.Fatalf("unexpected summary path %s", req.URL.Path)
						}
						body = `{"quoteSummary":{"result":[{"price":{"symbol":"` + tt.want + `"}}],"error":null}}`
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
				})
				for _, symbol := range []string{tt.input, tt.want} {
					var err error
					switch endpoint {
					case "chart":
						_, err = client.ChartTyped(context.Background(), symbol, ChartOptions{})
					case "quote":
						symbols := []string{symbol}
						_, err = client.Quote(context.Background(), symbols)
						if symbols[0] != symbol {
							t.Fatal("Quote mutated the caller's symbols")
						}
					case "qs":
						_, err = client.QuoteSummaryTyped(context.Background(), symbol, []QuoteSummaryModule{ModulePrice})
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				if calls != 1 {
					t.Fatalf("canonical and Yahoo symbols should share cache; got %d requests", calls)
				}
			})
		}
	}
}
