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
		if len(args) != 1 {
			return fmt.Errorf("symbol is required unless --list-modules is used")
		}
		symbol := args[0]

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
	// Set a price-only default format when the user hasn't specified one
	// via CLI flag, env (YF_FORMAT), or config file. This preserves the
	// global default elsewhere but makes `price` default to table output.
	priceCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if !cmd.Flags().Changed("format") && os.Getenv("YF_FORMAT") == "" && !viper.InConfig("format") {
			viper.Set("format", "table")
		}
	}
	rootCmd.AddCommand(priceCmd)
}
