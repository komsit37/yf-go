package cmd

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "sort"
    "strings"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "yf/internal/yfapi"
)

var (
    defaultModules = []string{
        "price",
        "summaryDetail",
        "financialData",
    }
    // Allowed modules as per refs/yahoo-finance2-2.x/docs/modules/quoteSummary.md
    allowedQuoteSummaryModules = []string{
        "assetProfile",
        "balanceSheetHistory",
        "balanceSheetHistoryQuarterly",
        "calendarEvents",
        "cashflowStatementHistory",
        "cashflowStatementHistoryQuarterly",
        "defaultKeyStatistics",
        "earnings",
        "earningsHistory",
        "earningsTrend",
        "financialData",
        "fundOwnership",
        "fundPerformance",
        "fundProfile",
        "incomeStatementHistory",
        "incomeStatementHistoryQuarterly",
        "indexTrend",
        "industryTrend",
        "insiderHolders",
        "insiderTransactions",
        "institutionOwnership",
        "majorDirectHolders",
        "majorHoldersBreakdown",
        "netSharePurchaseActivity",
        "price",
        "quoteType",
        "recommendationTrend",
        "secFilings",
        "sectorTrend",
        "summaryDetail",
        "summaryProfile",
        "symbol",
        "topHoldings",
        "upgradeDowngradeHistory",
    }
)

var qsCmd = &cobra.Command{
    Use:   "qs [symbol]",
    Short: "Get quote summary for a symbol",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // If listing modules, print and exit early
        if viper.GetBool("list-modules") {
            modules := append([]string(nil), allowedQuoteSummaryModules...)
            sort.Strings(modules)
            var out []byte
            var err error
            if viper.GetBool("pretty") {
                out, err = json.MarshalIndent(modules, "", "  ")
            } else {
                out, err = json.Marshal(modules)
            }
            if err != nil {
                return fmt.Errorf("failed to encode json: %w", err)
            }
            _, _ = os.Stdout.Write(out)
            if viper.GetBool("pretty") {
                _, _ = os.Stdout.WriteString("\n")
            }
            return nil
        }

        if len(args) != 1 {
            return fmt.Errorf("symbol is required unless --list-modules is used")
        }
        symbol := args[0]

        // Modules from flag/env; fallback to defaults
        modules := viper.GetStringSlice("modules")
        if len(modules) == 0 {
            modules = append([]string(nil), defaultModules...)
        }
        // Support comma-separated values passed as a single item (e.g., -m a,b,c)
        if len(modules) == 1 && strings.Contains(modules[0], ",") {
            modules = strings.Split(modules[0], ",")
        }
        // Trim whitespace from modules
        for i := range modules {
            modules[i] = strings.TrimSpace(modules[i])
        }
        // Validate requested modules
        if err := validateModules(modules, allowedQuoteSummaryModules); err != nil {
            return err
        }

        // Fetch data
        ctx := context.Background()
        result, err := yfapi.FetchQuoteSummary(ctx, symbol, modules)
        if err != nil {
            return err
        }

        // Output according to format
        switch viper.GetString("format") {
        case "json":
            var out []byte
            if viper.GetBool("pretty") {
                out, err = json.MarshalIndent(result, "", "  ")
            } else {
                out, err = json.Marshal(result)
            }
            if err != nil {
                return fmt.Errorf("failed to encode json: %w", err)
            }
            _, _ = os.Stdout.Write(out)
            if viper.GetBool("pretty") {
                _, _ = os.Stdout.WriteString("\n")
            }
            return nil
        default:
            return fmt.Errorf("unsupported format: %s", viper.GetString("format"))
        }
    },
}

func init() {
    rootCmd.AddCommand(qsCmd)
    // Add modules flag supporting repeats or comma separated values
    qsCmd.Flags().StringSliceP("modules", "m", defaultModules, "QuoteSummary modules (repeat or comma-separated). Use --modules multiple times or a,b,c")
    _ = viper.BindPFlag("modules", qsCmd.Flags().Lookup("modules"))
    // Add list-modules flag to print supported modules and exit without a symbol
    qsCmd.Flags().Bool("list-modules", false, "List supported quoteSummary modules and exit")
    _ = viper.BindPFlag("list-modules", qsCmd.Flags().Lookup("list-modules"))
}

// validateModules ensures all requested modules are supported; returns a helpful error otherwise.
func validateModules(requested, allowed []string) error {
    allowedSet := make(map[string]struct{}, len(allowed))
    for _, m := range allowed {
        allowedSet[m] = struct{}{}
    }
    var invalid []string
    for _, m := range requested {
        if _, ok := allowedSet[m]; !ok {
            invalid = append(invalid, m)
        }
    }
    if len(invalid) == 0 {
        return nil
    }
    sort.Strings(allowed)
    return fmt.Errorf("invalid module(s): %s. Allowed: %s", strings.Join(invalid, ", "), strings.Join(allowed, ", "))
}
