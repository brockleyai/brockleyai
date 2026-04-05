package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brockleyai/brockleyai/engine/graph"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/spf13/cobra"
)

var (
	validateFiles []string
	validateDirs  []string
)

var validateCmd = &cobra.Command{
	Use:   "validate [graph_id | -f file.json | -d dir/]",
	Short: "Validate a graph",
	Long: `Validate a graph either locally from a file or remotely via the API.

Local validation (no server required):
  brockley validate -f graph.json
  brockley validate -d graphs/

Remote validation (requires server):
  brockley validate graph_abc123`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringSliceVarP(&validateFiles, "file", "f", nil, "Graph JSON file(s) to validate locally")
	validateCmd.Flags().StringSliceVarP(&validateDirs, "dir", "d", nil, "Directory(ies) containing graph JSON files")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Local file validation
	if len(validateFiles) > 0 || len(validateDirs) > 0 {
		return validateLocal()
	}

	// Remote validation
	if len(args) == 0 {
		return fmt.Errorf("provide a graph_id, -f file.json, or -d directory")
	}

	c := newClient()
	result, err := c.ValidateGraph(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("validation request failed: %w", err)
	}

	return printValidationResult(result, args[0])
}

func validateLocal() error {
	files, err := collectGraphFiles(validateFiles, validateDirs)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no graph files found")
	}

	hasErrors := false
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", f, err)
			hasErrors = true
			continue
		}

		var g model.Graph
		if err := json.Unmarshal(data, &g); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", f, err)
			hasErrors = true
			continue
		}

		result := graph.Validate(&g)
		resultJSON, _ := json.Marshal(result)
		if err := printValidationResult(resultJSON, f); err != nil {
			return err
		}
		if !result.Valid {
			hasErrors = true
		}
	}

	if hasErrors {
		os.Exit(1)
	}
	return nil
}

func collectGraphFiles(files, dirs []string) ([]string, error) {
	var result []string
	result = append(result, files...)

	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && filepath.Ext(d.Name()) == ".json" {
				result = append(result, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
		}
	}
	return result, nil
}

func printValidationResult(data json.RawMessage, source string) error {
	if flagOutput == "json" {
		return printRawJSON(data)
	}

	var result struct {
		Valid  bool `json:"valid"`
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			NodeID  string `json:"node_id,omitempty"`
		} `json:"errors"`
		Warnings []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			NodeID  string `json:"node_id,omitempty"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return printRawJSON(data)
	}

	if result.Valid {
		fmt.Printf("✓ %s: valid", source)
		if len(result.Warnings) > 0 {
			fmt.Printf(" (%d warning(s))", len(result.Warnings))
		}
		fmt.Println()
	} else {
		fmt.Printf("✗ %s: invalid (%d error(s))\n", source, len(result.Errors))
	}

	for _, e := range result.Errors {
		loc := ""
		if e.NodeID != "" {
			loc = " [" + e.NodeID + "]"
		}
		fmt.Printf("  ERROR %s%s: %s\n", e.Code, loc, e.Message)
	}
	for _, w := range result.Warnings {
		loc := ""
		if w.NodeID != "" {
			loc = " [" + w.NodeID + "]"
		}
		fmt.Printf("  WARN  %s%s: %s\n", w.Code, loc, w.Message)
	}

	return nil
}
