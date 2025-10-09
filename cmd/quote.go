package cmd

import (
	"context"
	"fmt"
	"yf/pkg/yfapi"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// quoteCmd uses Yahoo Finance v7/finance/quote API to fetch quotes.
// Endpoint: https://query1.finance.yahoo.com/v7/finance/quote
// Supports multiple symbols via comma-separated input or multiple args.
var quoteCmd = &cobra.Command{
	Use:     "quote <symbol...>",
	Aliases: []string{"q"},
	Short:   "Get quotes via v7/finance/quote (raw JSON by default)",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		symbols := parseSymbols(args)
		if len(symbols) == 0 {
			return fmt.Errorf("at least one symbol is required")
		}

		// Actual runtime error: suppress usage output.
		cmd.SilenceUsage = true
		ctx := context.Background()
		quotes, err := yfapi.DefaultAPI.Quote(ctx, symbols)
		if err != nil {
			return err
		}
		switch viper.GetString("format") {
		case "json":
			if err := printJSON(quotes, viper.GetBool("pretty")); err != nil {
				return err
			}
			return nil
		case "table":
			// For now, only raw JSON is supported; suggest using `price` for tables.
			return fmt.Errorf("table output not supported for 'quote'; use 'price' or set -o json")
		default:
			return fmt.Errorf("unsupported format: %s", viper.GetString("format"))
		}
	},
}

func init() {
	rootCmd.AddCommand(quoteCmd)
	// Default format remains JSON (inherited from root), matching v7 raw output intent.
}
