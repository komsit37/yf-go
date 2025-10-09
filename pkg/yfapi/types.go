package yfapi

// YNum mirrors Yahoo's number objects which commonly contain raw/fmt/longFmt.
type YNum struct {
	Raw     *float64 `json:"raw"`
	Fmt     string   `json:"fmt"`
	LongFmt string   `json:"longFmt"`
}

type PriceModule struct {
    Symbol                     string `json:"symbol"`
    ShortName                  string `json:"shortName"`
    LongName                   string `json:"longName"`
    Currency                   string `json:"currency"`
    Exchange                   string `json:"exchange"`
    FullExchangeName           string `json:"fullExchangeName"`
    MarketState                string `json:"marketState"`
    ExchangeTimezoneName       string `json:"exchangeTimezoneName"`
    ExchangeTimezoneShortName  string `json:"exchangeTimezoneShortName"`
    RegularMarketChange        YNum   `json:"regularMarketChange"`
    RegularMarketPrice         YNum   `json:"regularMarketPrice"`
    RegularMarketChangePercent YNum   `json:"regularMarketChangePercent"`
    RegularMarketTime          int64  `json:"regularMarketTime"`
    RegularMarketVolume        YNum   `json:"regularMarketVolume"`
    AverageDailyVolume3Month   YNum   `json:"averageDailyVolume3Month"`
    MarketCap                  YNum   `json:"marketCap"`
    TrailingPE                 YNum   `json:"trailingPE"`
    RegularMarketPreviousClose YNum   `json:"regularMarketPreviousClose"`
}

type SummaryDetailModule struct {
	DividendYield    YNum `json:"dividendYield"`
	DividendRate     YNum `json:"dividendRate"`
	PayoutRatio      YNum `json:"payoutRatio"`
	FiftyTwoWeekLow  YNum `json:"fiftyTwoWeekLow"`
	FiftyTwoWeekHigh YNum `json:"fiftyTwoWeekHigh"`
	TrailingPE       YNum `json:"trailingPE"`
}

type FinancialDataModule struct {
	FinancialCurrency string `json:"financialCurrency"`

	GrossMargins     YNum `json:"grossMargins"`
	OperatingMargins YNum `json:"operatingMargins"`
	EbitdaMargins    YNum `json:"ebitdaMargins"`
	ProfitMargins    YNum `json:"profitMargins"`

	ReturnOnAssets YNum `json:"returnOnAssets"`
	ReturnOnEquity YNum `json:"returnOnEquity"`

	RevenueGrowth  YNum `json:"revenueGrowth"`
	EarningsGrowth YNum `json:"earningsGrowth"`

	CurrentRatio YNum `json:"currentRatio"`
	QuickRatio   YNum `json:"quickRatio"`
	DebtToEquity YNum `json:"debtToEquity"`

	TotalRevenue YNum `json:"totalRevenue"`
	Ebitda       YNum `json:"ebitda"`
	TotalCash    YNum `json:"totalCash"`
	TotalDebt    YNum `json:"totalDebt"`
	CashPerShare YNum `json:"totalCashPerShare"`
}

// AssetProfileModule captures a subset of common fields from the assetProfile module.
// This is intentionally lightweight; additional fields can be added as needed.
type AssetProfileModule struct {
	Sector              string `json:"sector"`
	Industry            string `json:"industry"`
	Website             string `json:"website"`
	LongBusinessSummary string `json:"longBusinessSummary"`
	Address1            string `json:"address1"`
	City                string `json:"city"`
	State               string `json:"state"`
	Country             string `json:"country"`
	FullTimeEmployees   int64  `json:"fullTimeEmployees"`
}

// QuoteSummaryTyped is a convenient typed view over commonly used modules.
// All modules are optional; fields are pointers and will be nil if the module
// was not requested or not present in the response.
// Warning: not tested
type QuoteSummaryTyped struct {
    Price         *PriceModule         `json:"price,omitempty"`
    SummaryDetail *SummaryDetailModule `json:"summaryDetail,omitempty"`
    FinancialData *FinancialDataModule `json:"financialData,omitempty"`
    AssetProfile  *AssetProfileModule  `json:"assetProfile,omitempty"`
}

// Quote represents a single entry from the v7/finance/quote endpoint
// (aka quoteResponse.result[]). Fields are a minimal subset commonly used
// by price displays. Values may be nil when unavailable.
type Quote struct {
    Symbol                     string   `json:"symbol"`
    ShortName                  string   `json:"shortName"`
    LongName                   string   `json:"longName"`
    Currency                   string   `json:"currency"`
    Exchange                   string   `json:"exchange"`
    FullExchangeName           string   `json:"fullExchangeName"`
    MarketState                string   `json:"marketState"`
    RegularMarketChange        *float64 `json:"regularMarketChange"`
    RegularMarketPrice         *float64 `json:"regularMarketPrice"`
    RegularMarketChangePercent *float64 `json:"regularMarketChangePercent"`
    RegularMarketTime          int64    `json:"regularMarketTime"`
    RegularMarketPreviousClose *float64 `json:"regularMarketPreviousClose"`
    RegularMarketVolume        *int64   `json:"regularMarketVolume"`
    AverageDailyVolume3Month   *int64   `json:"averageDailyVolume3Month"`
    MarketCap                  *int64   `json:"marketCap"`
    TrailingPE                 *float64 `json:"trailingPE"`
}
