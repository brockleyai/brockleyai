package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <resource> <id>",
	Short: "Inspect a resource in detail",
	Long:  `Fetch and display a resource in detail. Supports graphs, executions, schemas, prompt-templates, provider-configs, and api-tools.`,
}

var inspectGraphCmd = &cobra.Command{
	Use:     "graph <id>",
	Aliases: []string{"g"},
	Short:   "Inspect a graph",
	Args:    cobra.ExactArgs(1),
	RunE:    runInspectGraph,
}

var inspectExecutionCmd = &cobra.Command{
	Use:     "execution <id>",
	Aliases: []string{"exec", "e"},
	Short:   "Inspect an execution",
	Args:    cobra.ExactArgs(1),
	RunE:    runInspectExecution,
}

var inspectSchemaCmd = &cobra.Command{
	Use:     "schema <id>",
	Aliases: []string{"s"},
	Short:   "Inspect a schema",
	Args:    cobra.ExactArgs(1),
	RunE:    runInspectSchema,
}

var inspectPromptTemplateCmd = &cobra.Command{
	Use:     "prompt-template <id>",
	Aliases: []string{"pt"},
	Short:   "Inspect a prompt template",
	Args:    cobra.ExactArgs(1),
	RunE:    runInspectPromptTemplate,
}

var inspectProviderConfigCmd = &cobra.Command{
	Use:     "provider-config <id>",
	Aliases: []string{"pc"},
	Short:   "Inspect a provider config",
	Args:    cobra.ExactArgs(1),
	RunE:    runInspectProviderConfig,
}

var inspectAPIToolCmd = &cobra.Command{
	Use:     "api-tool <id>",
	Aliases: []string{"at"},
	Short:   "Inspect an API tool definition",
	Args:    cobra.ExactArgs(1),
	RunE:    runInspectAPITool,
}

var (
	inspectShowSteps bool
)

func init() {
	inspectExecutionCmd.Flags().BoolVar(&inspectShowSteps, "steps", false, "Include execution steps")
	inspectCmd.AddCommand(inspectGraphCmd, inspectExecutionCmd, inspectSchemaCmd, inspectPromptTemplateCmd, inspectProviderConfigCmd, inspectAPIToolCmd)
	rootCmd.AddCommand(inspectCmd)
}

func runInspectGraph(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.GetGraph(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to get graph: %w", err)
	}

	if flagOutput == "json" {
		return printRawJSON(result)
	}

	fmt.Printf("Graph: %s\n", extractString(result, "name"))
	fmt.Printf("  ID:        %s\n", extractString(result, "id"))
	fmt.Printf("  Status:    %s\n", extractString(result, "status"))
	fmt.Printf("  Version:   %s\n", extractString(result, "version"))
	fmt.Printf("  Namespace: %s\n", extractString(result, "namespace"))
	fmt.Printf("  Created:   %s\n", extractString(result, "created_at"))
	fmt.Printf("  Updated:   %s\n", extractString(result, "updated_at"))

	desc := extractString(result, "description")
	if desc != "" {
		fmt.Printf("  Desc:      %s\n", desc)
	}

	fmt.Println("\nFull JSON:")
	return printRawJSON(result)
}

func runInspectExecution(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.GetExecution(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	if flagOutput == "json" {
		if inspectShowSteps {
			steps, stepsErr := c.GetExecutionSteps(context.Background(), args[0])
			if stepsErr != nil {
				return fmt.Errorf("failed to get steps: %w", stepsErr)
			}
			combined := map[string]any{
				"execution": json.RawMessage(result),
				"steps":     steps,
			}
			return printJSON(combined)
		}
		return printRawJSON(result)
	}

	fmt.Printf("Execution: %s\n", extractString(result, "id"))
	fmt.Printf("  Graph:   %s\n", extractString(result, "graph_id"))
	fmt.Printf("  Status:  %s\n", extractString(result, "status"))
	fmt.Printf("  Trigger: %s\n", extractString(result, "trigger"))
	fmt.Printf("  Created: %s\n", extractString(result, "created_at"))

	if inspectShowSteps {
		steps, err := c.GetExecutionSteps(context.Background(), args[0])
		if err != nil {
			return fmt.Errorf("failed to get steps: %w", err)
		}
		fmt.Printf("\nSteps (%d):\n", len(steps.Items))
		w := newTabWriter()
		fmt.Fprintln(w, "  NODE\tTYPE\tSTATUS\tDURATION_MS")
		for _, s := range steps.Items {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
				extractString(s, "node_id"),
				extractString(s, "node_type"),
				extractString(s, "status"),
				extractString(s, "duration_ms"),
			)
		}
		return w.Flush()
	}

	fmt.Println("\nFull JSON:")
	return printRawJSON(result)
}

func runInspectSchema(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.GetSchema(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to get schema: %w", err)
	}
	return printRawJSON(result)
}

func runInspectPromptTemplate(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.GetPromptTemplate(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to get prompt template: %w", err)
	}
	return printRawJSON(result)
}

func runInspectProviderConfig(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.GetProviderConfig(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to get provider config: %w", err)
	}
	return printRawJSON(result)
}

func runInspectAPITool(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.GetAPITool(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to get API tool: %w", err)
	}
	return printRawJSON(result)
}
