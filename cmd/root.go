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
	rootCmd.PersistentFlags().StringP("format", "f", "table", "Output format (json|table)")
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
	moduleTTLs, err := moduleCacheTTLsFromConfig()
	if err != nil {
		return err
	}
	ttl := viper.GetDuration("cache-ttl")
	noCache := viper.GetBool("no-cache")
	wantCache := !noCache && (ttl > 0 || hasPositiveModuleTTL(moduleTTLs))
	opts := make([]yfgo.ClientOption, 0, 4)

	if wantCache {
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
		if len(moduleTTLs) > 0 {
			opts = append(opts, yfgo.WithQuoteSummaryModuleTTLs(moduleTTLs))
		}
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

func moduleCacheTTLsFromConfig() (map[yfgo.QuoteSummaryModule]time.Duration, error) {
	raw := viper.GetStringMap("cache.module-ttls")
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[yfgo.QuoteSummaryModule]time.Duration, len(raw))
	for key, val := range raw {
		mod, ok := yfgo.ParseQuoteSummaryModule(key)
		if !ok {
			return nil, fmt.Errorf("cache.module-ttls: unknown module %q", key)
		}
		ttl, err := parseDurationValue(val)
		if err != nil {
			return nil, fmt.Errorf("cache.module-ttls.%s: %w", mod.String(), err)
		}
		out[mod] = ttl
	}
	return out, nil
}

func hasPositiveModuleTTL(ttls map[yfgo.QuoteSummaryModule]time.Duration) bool {
	for _, ttl := range ttls {
		if ttl > 0 {
			return true
		}
	}
	return false
}

func parseDurationValue(raw any) (time.Duration, error) {
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, nil
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return d, nil
	case fmt.Stringer:
		return parseDurationValue(v.String())
	case int:
		return time.Duration(v) * time.Second, nil
	case int8:
		return time.Duration(v) * time.Second, nil
	case int16:
		return time.Duration(v) * time.Second, nil
	case int32:
		return time.Duration(v) * time.Second, nil
	case int64:
		return time.Duration(v) * time.Second, nil
	case uint:
		return time.Duration(v) * time.Second, nil
	case uint8:
		return time.Duration(v) * time.Second, nil
	case uint16:
		return time.Duration(v) * time.Second, nil
	case uint32:
		return time.Duration(v) * time.Second, nil
	case uint64:
		return time.Duration(v) * time.Second, nil
	case float32:
		return time.Duration(float64(time.Second) * float64(v)), nil
	case float64:
		return time.Duration(float64(time.Second) * v), nil
	default:
		return 0, fmt.Errorf("unsupported duration type %T", raw)
	}
}
