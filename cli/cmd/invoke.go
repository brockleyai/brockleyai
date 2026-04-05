package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/brockleyai/brockleyai/cli/client"
	"github.com/spf13/cobra"
)

var (
	invokeInput   string
	invokeFile    string
	invokeSync    bool
	invokeTimeout int
	invokePoll    bool
	invokeDebug   bool
)

var invokeCmd = &cobra.Command{
	Use:   "invoke <graph_id>",
	Short: "Invoke a graph execution",
	Long: `Invoke a graph execution by ID. Input can be provided inline or from a file.

Examples:
  brockley invoke graph_abc123 --input '{"text": "hello"}'
  brockley invoke graph_abc123 -f input.json --sync
  brockley invoke graph_abc123 --input '{"text": "hello"}' --poll`,
	Args: cobra.ExactArgs(1),
	RunE: runInvoke,
}

func init() {
	invokeCmd.Flags().StringVarP(&invokeInput, "input", "i", "", "JSON input (inline)")
	invokeCmd.Flags().StringVarP(&invokeFile, "input-file", "f", "", "JSON input file")
	invokeCmd.Flags().BoolVar(&invokeSync, "sync", false, "Wait for execution to complete (sync mode)")
	invokeCmd.Flags().IntVar(&invokeTimeout, "timeout", 0, "Timeout in seconds")
	invokeCmd.Flags().BoolVar(&invokePoll, "poll", false, "Poll for completion (async mode with polling)")
	invokeCmd.Flags().BoolVar(&invokeDebug, "debug", false, "Enable debug-only execution tracing")
	rootCmd.AddCommand(invokeCmd)
}

func runInvoke(cmd *cobra.Command, args []string) error {
	graphID := args[0]

	// Parse input
	var input any
	if invokeFile != "" {
		data, err := os.ReadFile(invokeFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("invalid JSON in input file: %w", err)
		}
	} else if invokeInput != "" {
		if err := json.Unmarshal([]byte(invokeInput), &input); err != nil {
			return fmt.Errorf("invalid JSON input: %w", err)
		}
	}

	mode := "async"
	if invokeSync {
		mode = "sync"
	}

	c := newClient()
	req := &client.InvokeRequest{
		GraphID: graphID,
		Input:   input,
		Mode:    mode,
		Timeout: invokeTimeout,
		Debug:   invokeDebug,
	}

	result, err := c.InvokeExecution(context.Background(), req)
	if err != nil {
		return fmt.Errorf("invocation failed: %w", err)
	}

	// If sync, just print result
	if invokeSync {
		return printRawJSON(result)
	}

	executionID := extractString(result, "id")
	if executionID == "" {
		executionID = extractString(result, "execution_id")
	}

	// If poll mode, keep polling until completion
	if invokePoll && executionID != "" {
		return pollExecution(c, executionID)
	}

	return printRawJSON(result)
}

func pollExecution(c *client.Client, executionID string) error {
	fmt.Fprintf(os.Stderr, "Polling execution %s...\n", executionID)
	for {
		result, err := c.GetExecution(context.Background(), executionID)
		if err != nil {
			return fmt.Errorf("polling failed: %w", err)
		}

		status := extractString(result, "status")
		switch status {
		case "completed", "failed", "cancelled", "timed_out":
			return printRawJSON(result)
		}

		time.Sleep(1 * time.Second)
	}
}
