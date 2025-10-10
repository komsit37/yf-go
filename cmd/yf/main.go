package main

import (
	"os"

	"github.com/komsit37/yf-go/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Cobra already prints the error; exit with non-zero status.
		os.Exit(1)
	}
}
