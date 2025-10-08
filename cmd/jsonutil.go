package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// printJSON writes the given value to stdout as JSON.
// When pretty is true, it attempts to pretty-print via `jq` if available,
// falling back to encoding/json's MarshalIndent when jq is not found or fails.
func printJSON(v any, pretty bool) error {
	// Fast path: compact JSON when not pretty
	if !pretty {
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to encode json: %w", err)
		}
		if _, err := os.Stdout.Write(b); err != nil {
			return err
		}
		return nil
	}

	// Pretty path: try jq first
	if _, err := exec.LookPath("jq"); err == nil {
		// Encode compact and let jq format it
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to encode json: %w", err)
		}
		cmd := exec.Command("jq", ".")
		cmd.Stdin = bytes.NewReader(b)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
		// If jq fails, fall back to MarshalIndent
	}

	// Fallback: MarshalIndent
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode json: %w", err)
	}
	if _, err := os.Stdout.Write(b); err != nil {
		return err
	}
	// Add trailing newline for readability when pretty
	_, _ = os.Stdout.WriteString("\n")
	return nil
}
