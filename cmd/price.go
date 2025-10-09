package cmd

import (
	"context"
	"fmt"
	"os"
	"yf/pkg/yfapi"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// priceCmd is a convenience command equivalent to:
//
//	yf qs <symbol> -m price
//
// When no symbol is provided, it defaults to 3353.T.
var priceCmd = &cobra.Command{
	Use:   "price [symbol]",
	Short: "Get price module for a symbol (shorthand for 'qs -m price')",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		symbol := "3353.T"
		if len(args) == 1 {
			symbol = args[0]
		}

		modules := []yfapi.QuoteSummaryModule{yfapi.ModulePrice}

		ctx := context.Background()
		result, err := yfapi.DefaultAPI.QuoteSummary(ctx, symbol, modules)
		if err != nil {
			return err
		}
		switch viper.GetString("format") {
		case "json":
			return printJSON(result, viper.GetBool("pretty"))
		case "table":
			renderSummary(os.Stdout, result)
			return nil
		default:
			return fmt.Errorf("unsupported format: %s", viper.GetString("format"))
		}
	},
}

func init() {
	rootCmd.AddCommand(priceCmd)
}
