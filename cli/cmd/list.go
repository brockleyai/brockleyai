package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	listNamespace string
	listStatus    string
	listLimit     int
	listGraphID   string // for executions
)

var listCmd = &cobra.Command{
	Use:   "list <resource>",
	Short: "List resources",
	Long:  `List graphs, executions, schemas, prompt-templates, provider-configs, or api-tools.`,
}

var listGraphsCmd = &cobra.Command{
	Use:     "graphs",
	Aliases: []string{"graph", "g"},
	Short:   "List graphs",
	RunE:    runListGraphs,
}

var listExecutionsCmd = &cobra.Command{
	Use:     "executions",
	Aliases: []string{"execution", "exec", "e"},
	Short:   "List executions",
	RunE:    runListExecutions,
}

var listSchemasCmd = &cobra.Command{
	Use:     "schemas",
	Aliases: []string{"schema", "s"},
	Short:   "List schemas",
	RunE:    runListSchemas,
}

var listPromptTemplatesCmd = &cobra.Command{
	Use:     "prompt-templates",
	Aliases: []string{"prompts", "pt"},
	Short:   "List prompt templates",
	RunE:    runListPromptTemplates,
}

var listProviderConfigsCmd = &cobra.Command{
	Use:     "provider-configs",
	Aliases: []string{"providers", "pc"},
	Short:   "List provider configs",
	RunE:    runListProviderConfigs,
}

var listAPIToolsCmd = &cobra.Command{
	Use:     "api-tools",
	Aliases: []string{"api-tool", "at"},
	Short:   "List API tool definitions",
	RunE:    runListAPITools,
}

func init() {
	listCmd.PersistentFlags().StringVar(&listNamespace, "namespace", "", "Filter by namespace")
	listCmd.PersistentFlags().IntVar(&listLimit, "limit", 20, "Max items to return")

	listGraphsCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (draft, active, archived)")
	listExecutionsCmd.Flags().StringVar(&listGraphID, "graph-id", "", "Filter by graph ID")
	listExecutionsCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")

	listCmd.AddCommand(listGraphsCmd, listExecutionsCmd, listSchemasCmd, listPromptTemplatesCmd, listProviderConfigsCmd, listAPIToolsCmd)
	rootCmd.AddCommand(listCmd)
}

func runListGraphs(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.ListGraphs(context.Background(), listNamespace, listStatus, "", listLimit)
	if err != nil {
		return fmt.Errorf("failed to list graphs: %w", err)
	}

	if flagOutput == "json" {
		return printJSON(result)
	}

	w := newTabWriter()
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tVERSION\tNAMESPACE")
	for _, item := range result.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			extractString(item, "id"),
			extractString(item, "name"),
			extractString(item, "status"),
			extractString(item, "version"),
			extractString(item, "namespace"),
		)
	}
	return w.Flush()
}

func runListExecutions(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.ListExecutions(context.Background(), listGraphID, listStatus, "", listLimit)
	if err != nil {
		return fmt.Errorf("failed to list executions: %w", err)
	}

	if flagOutput == "json" {
		return printJSON(result)
	}

	w := newTabWriter()
	fmt.Fprintln(w, "ID\tGRAPH_ID\tSTATUS\tTRIGGER\tCREATED_AT")
	for _, item := range result.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			extractString(item, "id"),
			extractString(item, "graph_id"),
			extractString(item, "status"),
			extractString(item, "trigger"),
			extractString(item, "created_at"),
		)
	}
	return w.Flush()
}

func runListSchemas(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.ListSchemas(context.Background(), listNamespace, "", listLimit)
	if err != nil {
		return fmt.Errorf("failed to list schemas: %w", err)
	}

	if flagOutput == "json" {
		return printJSON(result)
	}

	w := newTabWriter()
	fmt.Fprintln(w, "ID\tNAME\tNAMESPACE")
	for _, item := range result.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			extractString(item, "id"),
			extractString(item, "name"),
			extractString(item, "namespace"),
		)
	}
	return w.Flush()
}

func runListPromptTemplates(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.ListPromptTemplates(context.Background(), listNamespace, "", listLimit)
	if err != nil {
		return fmt.Errorf("failed to list prompt templates: %w", err)
	}

	if flagOutput == "json" {
		return printJSON(result)
	}

	w := newTabWriter()
	fmt.Fprintln(w, "ID\tNAME\tNAMESPACE\tFORMAT")
	for _, item := range result.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			extractString(item, "id"),
			extractString(item, "name"),
			extractString(item, "namespace"),
			extractString(item, "response_format"),
		)
	}
	return w.Flush()
}

func runListProviderConfigs(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.ListProviderConfigs(context.Background(), listNamespace, "", listLimit)
	if err != nil {
		return fmt.Errorf("failed to list provider configs: %w", err)
	}

	if flagOutput == "json" {
		return printJSON(result)
	}

	w := newTabWriter()
	fmt.Fprintln(w, "ID\tNAME\tPROVIDER\tNAMESPACE")
	for _, item := range result.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			extractString(item, "id"),
			extractString(item, "name"),
			extractString(item, "provider"),
			extractString(item, "namespace"),
		)
	}
	return w.Flush()
}

func runListAPITools(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.ListAPITools(context.Background(), listNamespace, "", listLimit)
	if err != nil {
		return fmt.Errorf("failed to list API tools: %w", err)
	}

	if flagOutput == "json" {
		return printJSON(result)
	}

	w := newTabWriter()
	fmt.Fprintln(w, "ID\tNAME\tNAMESPACE\tENDPOINTS\tBASE_URL")
	for _, item := range result.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			extractString(item, "id"),
			extractString(item, "name"),
			extractString(item, "namespace"),
			extractArrayLen(item, "endpoints"),
			extractString(item, "base_url"),
		)
	}
	return w.Flush()
}

// extractArrayLen returns the length of a JSON array field as a string.
func extractArrayLen(data json.RawMessage, key string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return "0"
	}
	arr, ok := m[key]
	if !ok {
		return "0"
	}
	var items []json.RawMessage
	if err := json.Unmarshal(arr, &items); err != nil {
		return "0"
	}
	return fmt.Sprintf("%d", len(items))
}
