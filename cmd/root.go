package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
    Use:   "yf",
    Short: "yf: Yahoo Finance CLI",
    Long:  "yf is a CLI to pull data from Yahoo Finance.",
}

// Execute runs the root command.
func Execute() error { return rootCmd.Execute() }

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringP("format", "o", "json", "Output format (json|table)")
	rootCmd.PersistentFlags().Bool("pretty", false, "Pretty print output")
	rootCmd.PersistentFlags().String("color", "auto", "Color output (auto|always|never)")

	// Bind to viper for future config/env usage
	_ = viper.BindPFlag("format", rootCmd.PersistentFlags().Lookup("format"))
	_ = viper.BindPFlag("pretty", rootCmd.PersistentFlags().Lookup("pretty"))
	_ = viper.BindPFlag("color", rootCmd.PersistentFlags().Lookup("color"))

	viper.SetEnvPrefix("YF")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Optional config file: $YF_CONFIG or yf.(yaml|json|toml) in CWD
	if cfg := os.Getenv("YF_CONFIG"); cfg != "" {
		viper.SetConfigFile(cfg)
		_ = viper.ReadInConfig()
	} else {
		viper.SetConfigName("yf")
		viper.AddConfigPath(".")
		_ = viper.ReadInConfig()
	}

	// Validate format early
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		format := strings.ToLower(viper.GetString("format"))
		switch format {
		case "json", "table":
			return nil
		default:
			return fmt.Errorf("unsupported format: %s (allowed: json, table)", format)
		}
	}
}
