package main

import (
	"os"
	"strings"

	"github.com/komsit37/yf-go/cmd"
)

func main() {
	os.Args = normalizeArgs(os.Args)
	if err := cmd.Execute(); err != nil {
		// Cobra already prints the error; exit with non-zero status.
		os.Exit(1)
	}
}

func normalizeArgs(args []string) []string {
	if len(args) <= 1 {
		return args
	}
	out := make([]string, 0, len(args))
	out = append(out, args[0])
	for _, arg := range args[1:] {
		switch {
		case arg == "-p1":
			out = append(out, "--period1")
		case strings.HasPrefix(arg, "-p1="):
			out = append(out, "--period1="+arg[len("-p1="):])
		case arg == "--p1":
			out = append(out, "--period1")
		case strings.HasPrefix(arg, "--p1="):
			out = append(out, "--period1="+arg[len("--p1="):])
		case arg == "-p2":
			out = append(out, "--period2")
		case strings.HasPrefix(arg, "-p2="):
			out = append(out, "--period2="+arg[len("-p2="):])
		case arg == "--p2":
			out = append(out, "--period2")
		case strings.HasPrefix(arg, "--p2="):
			out = append(out, "--period2="+arg[len("--p2="):])
		default:
			out = append(out, arg)
		}
	}
	return out
}
