package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	yfgo "github.com/komsit37/yf-go"
)

var sheetCmd = &cobra.Command{
	Use:   "sheet <symbol>",
	Short: "Render a compact security sheet with a chart and financial metrics",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := viper.GetString("sheet-render")
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("--render requires a PNG path or auto")
		}
		period1, err := parseChartTimestamp("period1", viper.GetString("sheet-period1"))
		if err != nil {
			return err
		}
		period2, err := parseChartTimestamp("period2", viper.GetString("sheet-period2"))
		if err != nil {
			return err
		}
		rangeValue := viper.GetString("sheet-range")
		if (period1 != nil || period2 != nil) && !cmd.Flags().Changed("range") {
			rangeValue = ""
		}
		cmd.SilenceUsage = true
		includePrePost := viper.GetBool("sheet-include-pre-post")
		path, err = yfgo.Default.SheetPNG(requestContext(cmd), args[0], yfgo.ChartOptions{
			Range: rangeValue, Interval: viper.GetString("sheet-interval"), Period1: period1, Period2: period2, IncludePrePost: &includePrePost,
		}, yfgo.SheetRenderOptions{
			RenderOptions: yfgo.RenderOptions{Path: path, Width: viper.GetInt("sheet-width"), Height: viper.GetInt("sheet-height")},
			FontSize:      viper.GetFloat64("sheet-font-size"), Theme: viper.GetString("sheet-theme"), Title: viper.GetString("sheet-title"),
		})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
		return err
	},
}

func init() {
	rootCmd.AddCommand(sheetCmd)
	sheetCmd.Flags().String("render", "auto", "PNG output path, or auto for a cached path (prints only the absolute path)")
	sheetCmd.Flags().Int("width", 800, "Sheet width in PNG pixels")
	sheetCmd.Flags().Int("height", 400, "Chart pane height in PNG pixels; text adds to the sheet height")
	sheetCmd.Flags().Float64("font-size", 22, "Text size in PNG pixels (22 is 11px when displayed at half size)")
	sheetCmd.Flags().String("theme", "light", "Text color for light or dark backgrounds; text background is transparent")
	sheetCmd.Flags().String("title", "", "Override the company name above the chart")
	sheetCmd.Flags().StringP("range", "r", "6mo", "Chart range (1d, 1mo, 3mo, 6mo, 1y, 5y, max, etc.)")
	sheetCmd.Flags().StringP("interval", "i", "1d", "Chart bar interval")
	sheetCmd.Flags().Bool("include-pre-post", true, "Include pre/post market data; disable for regular trading sessions only")
	sheetCmd.Flags().String("period1", "", "Custom range start (unix seconds or date)")
	sheetCmd.Flags().String("period2", "", "Custom range end (unix seconds or date)")
	for _, name := range []string{"render", "width", "height", "font-size", "theme", "title", "range", "interval", "include-pre-post", "period1", "period2"} {
		_ = viper.BindPFlag("sheet-"+name, sheetCmd.Flags().Lookup(name))
	}
}
