package cmd

import (
	"github.com/jedib0t/go-pretty/v6/table"
)

func applyTableStyle(t table.Writer) {
	t.SetStyle(table.StyleColoredDark)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateRows = false
	t.Style().Options.SeparateColumns = false
}
