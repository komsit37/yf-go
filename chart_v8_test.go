package yfgo

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// roundTripFunc lets tests inject canned HTTP responses without going over the network.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestChartSuccess(t *testing.T) {
	client := NewClient()
	client.sessionWarmed = true
	client.crumb = "crumb"
	called := false

	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		if got := req.URL.Host; got != "query1.finance.yahoo.com" {
			t.Fatalf("unexpected host %q", got)
		}
		values, _ := url.ParseQuery(req.URL.RawQuery)
		if values.Get("interval") != "1d" {
			t.Fatalf("interval = %q, want 1d", values.Get("interval"))
		}
		if values.Get("range") != "5d" {
			t.Fatalf("range = %q, want 5d", values.Get("range"))
		}
		if values.Get("includePrePost") != "false" {
			t.Fatalf("includePrePost = %q, want false", values.Get("includePrePost"))
		}
		if values.Get("return") != "object" {
			t.Fatalf("return = %q, want object", values.Get("return"))
		}
		body := `{"chart":{"result":[{"meta":{"symbol":"AAPL"},"timestamp":[1],"indicators":{}}],"error":null}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	ctx := context.Background()
	includePrePost := false
	useYfid := true
	opts := ChartOptions{
		Interval:       "1d",
		Range:          "5d",
		IncludePrePost: &includePrePost,
		UseYfid:        &useYfid,
		ReturnType:     "object",
	}
	res, err := client.Chart(ctx, "AAPL", opts)
	if err != nil {
		t.Fatalf("Chart() error = %v", err)
	}
	if !called {
		t.Fatalf("Chart() did not execute request")
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("Chart() result type = %T, want map[string]any", res)
	}
	meta, ok := m["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta type = %T", m["meta"])
	}
	if meta["symbol"] != "AAPL" {
		t.Fatalf("meta.symbol = %v, want AAPL", meta["symbol"])
	}
}

func TestChartTypedForcesObjectReturn(t *testing.T) {
	client := NewClient()
	client.sessionWarmed = true
	client.crumb = "crumb"
	var queriedReturn string
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		values, _ := url.ParseQuery(req.URL.RawQuery)
		queriedReturn = values.Get("return")
		body := `{"chart":{"result":[{"meta":{"symbol":"AAPL"},"timestamp":[1],"indicators":{"quote":[{"close":[1.23]}]}}],"error":null}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	ctx := context.Background()
	opts := ChartOptions{ReturnType: "array"}
	result, err := client.ChartTyped(ctx, "AAPL", opts)
	if err != nil {
		t.Fatalf("ChartTyped() error = %v", err)
	}
	if queriedReturn != "object" {
		t.Fatalf("ChartTyped should force return=object, got %q", queriedReturn)
	}
	if result.Meta.Symbol != "AAPL" {
		t.Fatalf("result.Meta.Symbol = %q, want AAPL", result.Meta.Symbol)
	}
	if len(result.Timestamp) != 1 || result.Timestamp[0] != 1 {
		t.Fatalf("result.Timestamp = %#v, want [1]", result.Timestamp)
	}
	if len(result.Indicators.Quote) != 1 {
		t.Fatalf("result.Indicators.Quote length = %d, want 1", len(result.Indicators.Quote))
	}
}

func TestChartAPIError(t *testing.T) {
	client := NewClient()
	client.sessionWarmed = true
	client.crumb = "crumb"
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"chart":{"result":[],"error":{"code":"NotFound","description":"No data found"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	ctx := context.Background()
	opts := ChartOptions{}
	if _, err := client.Chart(ctx, "XXXX", opts); err == nil {
		t.Fatalf("expected error from Chart()")
	}
}
