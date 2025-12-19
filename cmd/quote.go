package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	yfgo "github.com/komsit37/yf-go"
)

// quoteCmd uses Yahoo Finance v7/finance/quote API to fetch quotes.
// Endpoint: https://query1.finance.yahoo.com/v7/finance/quote
// Supports multiple symbols via comma-separated input or multiple args.
var quoteCmd = &cobra.Command{
	Use:     "quote <symbol...>",
	Aliases: []string{"q"},
	Short:   "Get quotes via v7/finance/quote (raw JSON output)",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		symbols := parseSymbols(args)
		if len(symbols) == 0 {
			return fmt.Errorf("at least one symbol is required")
		}

		// Actual runtime error: suppress usage output.
		cmd.SilenceUsage = true
		ctx := requestContext(cmd)
		quotes, err := yfgo.DefaultAPI.Quote(ctx, symbols)
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
			return fmt.Errorf("table output not supported for 'quote'; use 'price' or set -f json")
		default:
			return fmt.Errorf("unsupported format: %s", viper.GetString("format"))
		}
	},
}

func init() {
	rootCmd.AddCommand(quoteCmd)

	// Default quote output is JSON unless overridden via CLI flag, env (YF_FORMAT),
	// or config file.
	quoteCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if !cmd.Flags().Changed("format") && os.Getenv("YF_FORMAT") == "" && !viper.InConfig("format") {
			viper.Set("format", "json")
		}
	}
}
