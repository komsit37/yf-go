package cmd

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

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
)

var qsCmd = &cobra.Command{
    Use:   "qs [symbol]",
    Short: "Get quote summary for a symbol",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        symbol := args[0]

        // Fetch data
        ctx := context.Background()
        result, err := yfapi.FetchQuoteSummary(ctx, symbol, defaultModules)
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
}

