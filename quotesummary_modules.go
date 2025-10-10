package yfgo

import "strings"

// QuoteSummaryModule represents a supported quoteSummary module.
// Use the String() value when calling the upstream API.
type QuoteSummaryModule string

// String returns the canonical module name used by the API.
func (m QuoteSummaryModule) String() string { return string(m) }

// All supported modules as typed constants.
const (
	ModuleAssetProfile                 QuoteSummaryModule = "assetProfile"
	ModuleBalanceSheetHistory          QuoteSummaryModule = "balanceSheetHistory"
	ModuleBalanceSheetHistoryQuarterly QuoteSummaryModule = "balanceSheetHistoryQuarterly"
	ModuleCalendarEvents               QuoteSummaryModule = "calendarEvents"
	ModuleCashflowStatementHistory     QuoteSummaryModule = "cashflowStatementHistory"
	ModuleCashflowStatementHistoryQtr  QuoteSummaryModule = "cashflowStatementHistoryQuarterly"
	ModuleDefaultKeyStatistics         QuoteSummaryModule = "defaultKeyStatistics"
	ModuleEarnings                     QuoteSummaryModule = "earnings"
	ModuleEarningsHistory              QuoteSummaryModule = "earningsHistory"
	ModuleEarningsTrend                QuoteSummaryModule = "earningsTrend"
	ModuleFinancialData                QuoteSummaryModule = "financialData"
	ModuleFundOwnership                QuoteSummaryModule = "fundOwnership"
	ModuleFundPerformance              QuoteSummaryModule = "fundPerformance"
	ModuleFundProfile                  QuoteSummaryModule = "fundProfile"
	ModuleIncomeStatementHistory       QuoteSummaryModule = "incomeStatementHistory"
	ModuleIncomeStatementHistoryQtr    QuoteSummaryModule = "incomeStatementHistoryQuarterly"
	ModuleIndexTrend                   QuoteSummaryModule = "indexTrend"
	ModuleIndustryTrend                QuoteSummaryModule = "industryTrend"
	ModuleInsiderHolders               QuoteSummaryModule = "insiderHolders"
	ModuleInsiderTransactions          QuoteSummaryModule = "insiderTransactions"
	ModuleInstitutionOwnership         QuoteSummaryModule = "institutionOwnership"
	ModuleMajorDirectHolders           QuoteSummaryModule = "majorDirectHolders"
	ModuleMajorHoldersBreakdown        QuoteSummaryModule = "majorHoldersBreakdown"
	ModuleNetSharePurchaseActivity     QuoteSummaryModule = "netSharePurchaseActivity"
	ModulePrice                        QuoteSummaryModule = "price"
	ModuleQuoteType                    QuoteSummaryModule = "quoteType"
	ModuleRecommendationTrend          QuoteSummaryModule = "recommendationTrend"
	ModuleSecFilings                   QuoteSummaryModule = "secFilings"
	ModuleSectorTrend                  QuoteSummaryModule = "sectorTrend"
	ModuleSummaryDetail                QuoteSummaryModule = "summaryDetail"
	ModuleSummaryProfile               QuoteSummaryModule = "summaryProfile"
	ModuleSymbol                       QuoteSummaryModule = "symbol"
	ModuleTopHoldings                  QuoteSummaryModule = "topHoldings"
	ModuleUpgradeDowngradeHistory      QuoteSummaryModule = "upgradeDowngradeHistory"
)

// AllowedQuoteSummaryModules is the full list of modules supported by Yahoo's quoteSummary.
var AllowedQuoteSummaryModules = []QuoteSummaryModule{
	ModuleAssetProfile,
	ModuleBalanceSheetHistory,
	ModuleBalanceSheetHistoryQuarterly,
	ModuleCalendarEvents,
	ModuleCashflowStatementHistory,
	ModuleCashflowStatementHistoryQtr,
	ModuleDefaultKeyStatistics,
	ModuleEarnings,
	ModuleEarningsHistory,
	ModuleEarningsTrend,
	ModuleFinancialData,
	ModuleFundOwnership,
	ModuleFundPerformance,
	ModuleFundProfile,
	ModuleIncomeStatementHistory,
	ModuleIncomeStatementHistoryQtr,
	ModuleIndexTrend,
	ModuleIndustryTrend,
	ModuleInsiderHolders,
	ModuleInsiderTransactions,
	ModuleInstitutionOwnership,
	ModuleMajorDirectHolders,
	ModuleMajorHoldersBreakdown,
	ModuleNetSharePurchaseActivity,
	ModulePrice,
	ModuleQuoteType,
	ModuleRecommendationTrend,
	ModuleSecFilings,
	ModuleSectorTrend,
	ModuleSummaryDetail,
	ModuleSummaryProfile,
	ModuleSymbol,
	ModuleTopHoldings,
	ModuleUpgradeDowngradeHistory,
}

// DefaultQuoteSummaryModules is the minimal set used by table rendering.
var DefaultQuoteSummaryModules = []QuoteSummaryModule{
	ModulePrice,
	ModuleSummaryDetail,
	ModuleFinancialData,
}

// ModulesToStrings converts typed modules to their canonical string names.
func ModulesToStrings(mods []QuoteSummaryModule) []string {
	out := make([]string, len(mods))
	for i, m := range mods {
		out[i] = m.String()
	}
	return out
}

// moduleAliases defines accepted aliases for each module for human-friendly parsing.
// The canonical module name is always accepted, along with case-insensitive and
// separator-insensitive forms (handled by normalization). Some common shorthands
// are provided for convenience (e.g., "p" => price, "sd" => summaryDetail).
var moduleAliases = map[QuoteSummaryModule][]string{
	ModuleAssetProfile:                 {"asset", "profile", "ap"},
	ModuleBalanceSheetHistory:          {"balancesheet", "bsh"},
	ModuleBalanceSheetHistoryQuarterly: {"balancesheetquarterly", "bshq"},
	ModuleCalendarEvents:               {"calendar", "events"},
	ModuleCashflowStatementHistory:     {"cashflow", "cfh"},
	ModuleCashflowStatementHistoryQtr:  {"cashflowquarterly", "cfhq"},
	ModuleDefaultKeyStatistics:         {"keystats", "dks", "stats"},
	ModuleEarnings:                     {"earn"},
	ModuleEarningsHistory:              {"earninghistory", "eh"},
	ModuleEarningsTrend:                {"earningstrend", "et"},
	ModuleFinancialData:                {"financial", "fd"},
	ModuleFundOwnership:                {"fundowners", "fo"},
	ModuleFundPerformance:              {"fundperf", "fp"},
	ModuleFundProfile:                  {"fund", "fundprof"},
	ModuleIncomeStatementHistory:       {"incomestatement", "ish"},
	ModuleIncomeStatementHistoryQtr:    {"incomestatementquarterly", "ishq"},
	ModuleIndexTrend:                   {"index", "it"},
	ModuleIndustryTrend:                {"industry", "indt"},
	ModuleInsiderHolders:               {"insiders", "ih"},
	ModuleInsiderTransactions:          {"insiderx", "itx", "insidertransactions"},
	ModuleInstitutionOwnership:         {"institutions", "instown"},
	ModuleMajorDirectHolders:           {"majordirect", "mdh"},
	ModuleMajorHoldersBreakdown:        {"majorholders", "mhb"},
	ModuleNetSharePurchaseActivity:     {"netsharepurchase", "nspa"},
	ModulePrice:                        {"p"},
	ModuleQuoteType:                    {"type", "qt"},
	ModuleRecommendationTrend:          {"reco", "rt"},
	ModuleSecFilings:                   {"sec", "filings"},
	ModuleSectorTrend:                  {"sector", "sect"},
	ModuleSummaryDetail:                {"summary", "sd"},
	ModuleSummaryProfile:               {"summaryprofile", "sp"},
	ModuleSymbol:                       {"sym"},
	ModuleTopHoldings:                  {"tophold", "th"},
	ModuleUpgradeDowngradeHistory:      {"updown", "udh"},
}

// moduleAliasIndex maps normalized alias => module. Built once at init.
var moduleAliasIndex map[string]QuoteSummaryModule

func init() {
	moduleAliasIndex = make(map[string]QuoteSummaryModule, len(AllowedQuoteSummaryModules)*2)
	for _, m := range AllowedQuoteSummaryModules {
		// Always accept the canonical name
		moduleAliasIndex[norm(string(m))] = m
		// And any provided aliases
		for _, a := range moduleAliases[m] {
			moduleAliasIndex[norm(a)] = m
		}
	}
}

// ParseQuoteSummaryModule parses a raw string into a typed module, supporting
// case-insensitive and separator-agnostic matches, plus defined shorthands.
// Returns the module and true if recognized.
func ParseQuoteSummaryModule(s string) (QuoteSummaryModule, bool) {
	if m, ok := moduleAliasIndex[norm(s)]; ok {
		return m, true
	}
	return "", false
}

// norm normalizes a string for alias comparison: lowercased and with
// dashes/underscores/spaces removed.
func norm(s string) string {
	s = strings.ToLower(s)
	// Remove some common separators
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(s)
}
