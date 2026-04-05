// Package cmd implements the brockley CLI commands.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/brockleyai/brockleyai/cli/client"
	"github.com/spf13/cobra"
)

var (
	flagServerURL string
	flagAPIKey    string
	flagOutput    string // "json" or "table"
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "brockley",
	Short: "Brockley CLI — manage agent workflows",
	Long:  `Brockley is an open-source AI agent infrastructure platform. This CLI lets you validate, invoke, inspect, and export agent graphs.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagServerURL, "server", envOrDefault("BROCKLEY_SERVER_URL", "http://localhost:8000"), "Brockley server URL")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", os.Getenv("BROCKLEY_API_KEY"), "API key for authentication")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table", "Output format: json or table")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newClient() *client.Client {
	return client.New(flagServerURL, flagAPIKey)
}

// printJSON pretty-prints JSON to stdout.
func printJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// printRawJSON pretty-prints raw JSON bytes to stdout.
func printRawJSON(data json.RawMessage) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		_, writeErr := os.Stdout.Write(data)
		return writeErr
	}
	return printJSON(v)
}

// newTabWriter creates a tabwriter for table output.
func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

// extractString extracts a string from a JSON object field.
func extractString(data json.RawMessage, key string) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case float64:
			return fmt.Sprintf("%.0f", val)
		default:
			return fmt.Sprintf("%v", val)
		}
	}
	return ""
}
