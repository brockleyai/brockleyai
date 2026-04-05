package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOut    string
)

var exportCmd = &cobra.Command{
	Use:   "export <graph_id | -f file.json>",
	Short: "Export a graph in JSON, YAML, or Terraform HCL format",
	Long: `Export a graph definition. Fetches from server by ID or reads from a local file.

Supported formats:
  json       - JSON (default)
  yaml       - YAML
  terraform  - Terraform HCL resource block

Examples:
  brockley export graph_abc123
  brockley export graph_abc123 --format terraform
  brockley export -f graph.json --format terraform --out main.tf`,
	RunE: runExport,
}

var exportFiles []string

func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "json", "Export format: json, yaml, terraform")
	exportCmd.Flags().StringVar(&exportOut, "out", "", "Output file (default: stdout)")
	exportCmd.Flags().StringSliceVarP(&exportFiles, "file", "f", nil, "Local graph file(s) to export")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	var graphData json.RawMessage

	if len(exportFiles) > 0 {
		// Read local file
		data, err := os.ReadFile(exportFiles[0])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		graphData = data
	} else if len(args) > 0 {
		// Fetch from server
		c := newClient()
		result, err := c.GetGraph(context.Background(), args[0])
		if err != nil {
			return fmt.Errorf("failed to get graph: %w", err)
		}
		graphData = result
	} else {
		return fmt.Errorf("provide a graph_id or -f file.json")
	}

	var output string
	var err error

	switch exportFormat {
	case "json":
		var v any
		json.Unmarshal(graphData, &v)
		b, _ := json.MarshalIndent(v, "", "  ")
		output = string(b) + "\n"
	case "yaml":
		output, err = exportToYAML(graphData)
	case "terraform":
		output, err = exportToTerraform(graphData)
	default:
		return fmt.Errorf("unsupported format: %s (use json, yaml, or terraform)", exportFormat)
	}

	if err != nil {
		return err
	}

	if exportOut != "" {
		return os.WriteFile(exportOut, []byte(output), 0644)
	}
	fmt.Print(output)
	return nil
}

func exportToYAML(data json.RawMessage) (string, error) {
	var g model.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return "", fmt.Errorf("failed to parse graph: %w", err)
	}

	// Simple YAML-like output (we avoid a YAML dependency by generating manually)
	var out string
	out += fmt.Sprintf("name: %s\n", g.Name)
	out += fmt.Sprintf("namespace: %s\n", g.Namespace)
	if g.Description != "" {
		out += fmt.Sprintf("description: %s\n", g.Description)
	}
	out += fmt.Sprintf("status: %s\n", g.Status)
	out += fmt.Sprintf("version: %d\n", g.Version)

	out += "nodes:\n"
	for _, n := range g.Nodes {
		out += fmt.Sprintf("  - id: %s\n", n.ID)
		out += fmt.Sprintf("    name: %s\n", n.Name)
		out += fmt.Sprintf("    type: %s\n", n.Type)
		if len(n.InputPorts) > 0 {
			out += "    input_ports:\n"
			for _, p := range n.InputPorts {
				out += fmt.Sprintf("      - name: %s\n", p.Name)
				out += fmt.Sprintf("        schema: %s\n", string(p.Schema))
			}
		}
		if len(n.OutputPorts) > 0 {
			out += "    output_ports:\n"
			for _, p := range n.OutputPorts {
				out += fmt.Sprintf("      - name: %s\n", p.Name)
				out += fmt.Sprintf("        schema: %s\n", string(p.Schema))
			}
		}
		if len(n.Config) > 0 && string(n.Config) != "{}" {
			out += fmt.Sprintf("    config: %s\n", string(n.Config))
		}
	}

	out += "edges:\n"
	for _, e := range g.Edges {
		out += fmt.Sprintf("  - id: %s\n", e.ID)
		out += fmt.Sprintf("    source_node_id: %s\n", e.SourceNodeID)
		out += fmt.Sprintf("    source_port: %s\n", e.SourcePort)
		out += fmt.Sprintf("    target_node_id: %s\n", e.TargetNodeID)
		out += fmt.Sprintf("    target_port: %s\n", e.TargetPort)
		if e.BackEdge {
			out += "    back_edge: true\n"
			if e.Condition != "" {
				out += fmt.Sprintf("    condition: %s\n", e.Condition)
			}
		}
	}

	return out, nil
}

func exportToTerraform(data json.RawMessage) (string, error) {
	var g model.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return "", fmt.Errorf("failed to parse graph: %w", err)
	}

	resourceName := sanitizeTerraformName(g.Name)

	nodesJSON, _ := json.MarshalIndent(g.Nodes, "    ", "  ")
	edgesJSON, _ := json.MarshalIndent(g.Edges, "    ", "  ")

	var out string
	out += fmt.Sprintf("resource \"brockley_graph\" %q {\n", resourceName)
	out += fmt.Sprintf("  name        = %q\n", g.Name)
	out += fmt.Sprintf("  namespace   = %q\n", g.Namespace)
	if g.Description != "" {
		out += fmt.Sprintf("  description = %q\n", g.Description)
	}
	out += fmt.Sprintf("  status      = %q\n", g.Status)
	out += fmt.Sprintf("\n  nodes = jsonencode(%s)\n", string(nodesJSON))
	out += fmt.Sprintf("\n  edges = jsonencode(%s)\n", string(edgesJSON))

	if g.State != nil {
		stateJSON, _ := json.MarshalIndent(g.State, "    ", "  ")
		out += fmt.Sprintf("\n  state = jsonencode(%s)\n", string(stateJSON))
	}

	out += "}\n"

	return out, nil
}

func sanitizeTerraformName(name string) string {
	var result []byte
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result = append(result, byte(c))
		} else {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "graph"
	}
	// Ensure starts with letter or underscore
	if result[0] >= '0' && result[0] <= '9' {
		result = append([]byte("_"), result...)
	}
	return string(result)
}
