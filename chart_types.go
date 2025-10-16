package yfgo

// ChartResult models the normalized structure returned by Yahoo's chart API
// when using return=object. Arrays may contain nil values when data is missing.
type ChartResult struct {
	Meta       ChartMeta       `json:"meta"`
	Timestamp  []int64         `json:"timestamp,omitempty"`
	Indicators ChartIndicators `json:"indicators,omitempty"`
	Events     ChartEvents     `json:"events,omitempty"`
	Quotes     []ChartQuote    `json:"quotes,omitempty"` // present when return=array
}

// ChartMeta captures summary metadata for a chart response.
type ChartMeta struct {
	Currency              string            `json:"currency"`
	Symbol                string            `json:"symbol"`
	ExchangeName          string            `json:"exchangeName"`
	InstrumentType        string            `json:"instrumentType"`
	FirstTradeDate        int64             `json:"firstTradeDate"`
	RegularMarketTime     int64             `json:"regularMarketTime"`
	GmtOffset             int64             `json:"gmtoffset"`
	Timezone              string            `json:"timezone"`
	ExchangeTimezoneName  string            `json:"exchangeTimezoneName"`
	RegularMarketPrice    *float64          `json:"regularMarketPrice"`
	ChartPreviousClose    *float64          `json:"chartPreviousClose"`
	PriceHint             *int64            `json:"priceHint"`
	DataGranularity       string            `json:"dataGranularity"`
	Range                 string            `json:"range"`
	ValidRanges           []string          `json:"validRanges"`
	HasPrePostMarketData  *bool             `json:"hasPrePostMarketData,omitempty"`
	RegularMarketDayHigh  *float64          `json:"regularMarketDayHigh,omitempty"`
	RegularMarketDayLow   *float64          `json:"regularMarketDayLow,omitempty"`
	RegularMarketVolume   *int64            `json:"regularMarketVolume,omitempty"`
	LongName              string            `json:"longName,omitempty"`
	ShortName             string            `json:"shortName,omitempty"`
	CurrentTradingPeriod  *TradingPeriodSet `json:"currentTradingPeriod,omitempty"`
	IndicatorsGranularity string            `json:"indicatorsGranularity,omitempty"`
}

// TradingPeriodSet groups Yahoo's pre/regular/post sessions.
type TradingPeriodSet struct {
	Pre     *TradingPeriod `json:"pre,omitempty"`
	Regular *TradingPeriod `json:"regular,omitempty"`
	Post    *TradingPeriod `json:"post,omitempty"`
}

// TradingPeriod describes a single trading session window.
type TradingPeriod struct {
	Timezone  string `json:"timezone"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	GmtOffset int64  `json:"gmtoffset"`
}

// ChartIndicators contains numeric series for the selected interval.
type ChartIndicators struct {
	Quote    []ChartQuoteSeries    `json:"quote,omitempty"`
	AdjClose []ChartAdjCloseSeries `json:"adjclose,omitempty"`
}

// ChartQuoteSeries hosts OHLCV arrays. Yahoo typically returns a single entry.
type ChartQuoteSeries struct {
	Open   []*float64 `json:"open,omitempty"`
	High   []*float64 `json:"high,omitempty"`
	Low    []*float64 `json:"low,omitempty"`
	Close  []*float64 `json:"close,omitempty"`
	Volume []*int64   `json:"volume,omitempty"`
}

// ChartAdjCloseSeries hosts adjusted close data when requested.
type ChartAdjCloseSeries struct {
	AdjClose []*float64 `json:"adjclose,omitempty"`
}

// ChartEvents includes optional dividend, split, and earnings events keyed by timestamp.
type ChartEvents struct {
	Dividends map[string]ChartDividend `json:"dividends,omitempty"`
	Splits    map[string]ChartSplit    `json:"splits,omitempty"`
	Earnings  map[string]ChartEarning  `json:"earnings,omitempty"`
}

// ChartDividend describes a dividend event.
type ChartDividend struct {
	Amount *float64 `json:"amount,omitempty"`
	Date   int64    `json:"date"`
}

// ChartSplit describes a split event.
type ChartSplit struct {
	Date        int64  `json:"date"`
	Numerator   *int64 `json:"numerator,omitempty"`
	Denominator *int64 `json:"denominator,omitempty"`
	SplitRatio  string `json:"splitRatio,omitempty"`
}

// ChartEarning describes an earnings event when present.
type ChartEarning struct {
	Date               int64    `json:"date"`
	EPSActual          *float64 `json:"earningsActual,omitempty"`
	EPSEstimate        *float64 `json:"earningsEstimate,omitempty"`
	RevenueActual      *float64 `json:"revActual,omitempty"`
	RevenueEstimate    *float64 `json:"revEstimate,omitempty"`
	ReportedCurrency   string   `json:"currency,omitempty"`
	ReportedFinancials string   `json:"reportedFinancials,omitempty"`
}

// ChartQuote is emitted by Yahoo when using return=array for convenience.
type ChartQuote struct {
	Date     int64    `json:"date"`
	High     *float64 `json:"high,omitempty"`
	Volume   *int64   `json:"volume,omitempty"`
	Open     *float64 `json:"open,omitempty"`
	Low      *float64 `json:"low,omitempty"`
	Close    *float64 `json:"close,omitempty"`
	AdjClose *float64 `json:"adjclose,omitempty"`
}
