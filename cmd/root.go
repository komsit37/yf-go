package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	yfgo "github.com/komsit37/yf-go"
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

	rootCmd.PersistentFlags().Duration("cache-ttl", 5*time.Minute, "Cache TTL (set to 0 to disable caching)")
	_ = viper.BindPFlag("cache-ttl", rootCmd.PersistentFlags().Lookup("cache-ttl"))

	rootCmd.PersistentFlags().String("cache-dir", "", "Directory for on-disk cache (defaults to $YF_HOME/cache)")
	_ = viper.BindPFlag("cache-dir", rootCmd.PersistentFlags().Lookup("cache-dir"))

	rootCmd.PersistentFlags().Bool("force-refresh", false, "Force refresh cached data for this invocation")
	_ = viper.BindPFlag("force-refresh", rootCmd.PersistentFlags().Lookup("force-refresh"))

	rootCmd.PersistentFlags().Bool("no-cache", false, "Bypass caches for this invocation")
	_ = viper.BindPFlag("no-cache", rootCmd.PersistentFlags().Lookup("no-cache"))

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
		default:
			return fmt.Errorf("unsupported format: %s (allowed: json, table)", format)
		}
		return configureClient()
	}
}

func configureClient() error {
	ttl := viper.GetDuration("cache-ttl")
	noCache := viper.GetBool("no-cache")
	opts := make([]yfgo.ClientOption, 0, 3)

	if !noCache && ttl > 0 {
		opts = append(opts, yfgo.WithDefaultCacheTTL(ttl))
		dir := strings.TrimSpace(viper.GetString("cache-dir"))
		if dir == "" {
			var err error
			dir, err = defaultCacheDir()
			if err != nil {
				return err
			}
		}
		store, err := yfgo.NewFileCacheStore(dir)
		if err != nil {
			return err
		}
		opts = append(opts, yfgo.WithCacheStore(store))
	} else {
		opts = append(opts, yfgo.WithCacheDisabled())
	}

	client := yfgo.NewClient(opts...)
	yfgo.Default = client
	yfgo.DefaultAPI = client
	return nil
}

func defaultCacheDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv("YF_HOME")); home != "" {
		return filepath.Join(home, "cache"), nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if userHome == "" {
		return "", fmt.Errorf("user home directory not found")
	}
	return filepath.Join(userHome, ".yf", "cache"), nil
}
