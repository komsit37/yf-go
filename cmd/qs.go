package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	yfgo "github.com/pkomsit/yf-go"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Supports multiple symbols via comma-separated input or multiple args.
var qsCmd = &cobra.Command{
	Use:   "qs <symbol...>",
	Short: "Get quote summary for one or more symbols",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// If listing modules, print and exit early
		if viper.GetBool("list-modules") {
			// Build canonical module names from typed list
			names := make([]string, 0, len(yfgo.AllowedQuoteSummaryModules))
			for _, m := range yfgo.AllowedQuoteSummaryModules {
				names = append(names, m.String())
			}
			sort.Strings(names)
			if err := printJSON(names, viper.GetBool("pretty")); err != nil {
				return err
			}
			return nil
		}

		// Parse symbols from args supporting comma-separated values
		symbols := parseSymbols(args)
		if len(symbols) == 0 {
			return fmt.Errorf("symbol is required unless --list-modules is used")
		}

		// Resolve modules from flags/env, supporting comma-separated values and aliases
		typedMods, err := resolveModules(viper.GetStringSlice("modules"))
		if err != nil {
			// Argument/config error: allow usage to show.
			return err
		}

		// arg parsing done, suppress usage on error after this point
		cmd.SilenceUsage = true
		// Fetch data per symbol
		ctx := context.Background()
		results := make([]any, 0, len(symbols))
		for _, sym := range symbols {
			res, err := yfgo.DefaultAPI.QuoteSummary(ctx, sym, typedMods)
			if err != nil {
				// Runtime/API error: suppress usage output.
				return err
			}
			results = append(results, res)
		}

		switch viper.GetString("format") {
		case "json":
			if len(results) == 1 {
				if err := printJSON(results[0], viper.GetBool("pretty")); err != nil {
					return err
				}
				return nil
			}
			if err := printJSON(results, viper.GetBool("pretty")); err != nil {
				return err
			}
			return nil
		case "table":
			for i, sym := range symbols {
				if i > 0 {
					fmt.Fprintln(os.Stdout, "")
				}
				// Print a simple symbol heading before its modules
				fmt.Fprintln(os.Stdout, strings.ToUpper(sym))
				renderSummary(os.Stdout, results[i])
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
	qsCmd.Flags().StringSliceP("modules", "m", yfgo.ModulesToStrings(yfgo.DefaultQuoteSummaryModules), "QuoteSummary modules (repeat or comma-separated). Use --modules multiple times or a,b,c")
	_ = viper.BindPFlag("modules", qsCmd.Flags().Lookup("modules"))
	// Add list-modules flag to print supported modules and exit without a symbol
	qsCmd.Flags().Bool("list-modules", false, "List supported quoteSummary modules and exit")
	_ = viper.BindPFlag("list-modules", qsCmd.Flags().Lookup("list-modules"))
}

// validateModules ensures all requested modules are supported; returns a helpful error otherwise.
func parseAndValidateModules(requested []string) ([]yfgo.QuoteSummaryModule, error) {
	var out []yfgo.QuoteSummaryModule
	var invalid []string
	for _, r := range requested {
		if m, ok := yfgo.ParseQuoteSummaryModule(r); ok {
			out = append(out, m)
		} else {
			invalid = append(invalid, r)
		}
	}
	if len(invalid) == 0 {
		return out, nil
	}
	// Build allowed canonical names for the error message
	allowed := make([]string, 0, len(yfgo.AllowedQuoteSummaryModules))
	for _, m := range yfgo.AllowedQuoteSummaryModules {
		allowed = append(allowed, m.String())
	}
	sort.Strings(allowed)
	return nil, fmt.Errorf("invalid module(s): %s. Allowed: %s", strings.Join(invalid, ", "), strings.Join(allowed, ", "))
}

// resolveModules converts raw module strings into typed modules.
// It supports comma-separated values, trims whitespace, and falls back to
// defaults when the provided slice is empty.
func resolveModules(raw []string) ([]yfgo.QuoteSummaryModule, error) {
	// Flatten any comma-separated entries
	var requested []string
	for _, item := range raw {
		for _, seg := range strings.Split(item, ",") {
			if s := strings.TrimSpace(seg); s != "" {
				requested = append(requested, s)
			}
		}
	}
	if len(requested) == 0 {
		// Use typed defaults directly
		return append([]yfgo.QuoteSummaryModule(nil), yfgo.DefaultQuoteSummaryModules...), nil
	}
	return parseAndValidateModules(requested)
}

// ---- Rendering (table) ----

// Render helpers are CLI concerns; types are from the root package.
// ---- Generic rendering (table output) ----

// renderSummary prints the untyped quoteSummary result in a human-friendly, generic form.
// It renders top-level modules as sections, and pretty-prints nested objects/arrays.
func renderSummary(w *os.File, v any) {
	switch root := v.(type) {
	case map[string]any:
		// Render modules in sorted order
		keys := make([]string, 0, len(root))
		for k := range root {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				fmt.Fprintln(w, sepLine())
			}
			fmt.Fprintln(w, strings.ToUpper(k))
			renderAny(w, root[k], 2)
		}
	default:
		// Fallback: just print recursively
		renderAny(w, v, 0)
	}
}

func renderAny(w *os.File, v any, indent int) {
	switch val := v.(type) {
	case map[string]any:
		// Detect Yahoo number object {raw, fmt, longFmt}
		if s, ok := ynumDisplay(val); ok {
			fmt.Fprintln(w, indentStr(indent)+s)
			return
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vv := val[k]
			switch vv.(type) {
			case map[string]any, []any:
				fmt.Fprintf(w, "%s%s:\n", indentStr(indent), k)
				renderAny(w, vv, indent+2)
			default:
				fmt.Fprintf(w, "%s%s: %s\n", indentStr(indent), k, scalar(vv))
			}
		}
	case []any:
		for i, item := range val {
			prefix := fmt.Sprintf("%s- ", indentStr(indent))
			switch item.(type) {
			case map[string]any, []any:
				fmt.Fprintf(w, "%s[%d]\n", indentStr(indent), i)
				renderAny(w, item, indent+2)
			default:
				fmt.Fprintf(w, "%s%s\n", prefix, scalar(item))
			}
		}
	default:
		fmt.Fprintln(w, indentStr(indent)+scalar(val))
	}
}

func ynumDisplay(m map[string]any) (string, bool) {
	// Prefer fmt if present
	if v, ok := m["fmt"]; ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			return s, true
		}
	}
	if v, ok := m["raw"]; ok {
		switch t := v.(type) {
		case float64:
			return trimZeros(fmtFloat(t)), true
		case string:
			return t, true
		}
	}
	return "", false
}

func indentStr(n int) string { return strings.Repeat(" ", n) }

func scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return "-"
	case string:
		if strings.TrimSpace(t) == "" {
			return "-"
		}
		return t
	case float64:
		return trimZeros(fmtFloat(t))
	default:
		return fmt.Sprint(t)
	}
}

// fmtFloat and trimZeros are small helpers for compact numeric display.
func fmtFloat(f float64) string { return fmt.Sprintf("%.3f", f) }

func trimZeros(s string) string {
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

func sepLine() string { return strings.Repeat("─", 62) }
