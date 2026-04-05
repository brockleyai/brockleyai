package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/brockleyai/brockleyai/cli/client"
	"github.com/spf13/cobra"
)

var (
	deployFiles     []string
	deployDirs      []string
	deployNamespace string
	deployEnvFile   string
)

var deployCmd = &cobra.Command{
	Use:   "deploy [-f file.json | -d dir/]",
	Short: "Deploy graphs and API tool definitions to the server",
	Long: `Push graph and API tool definitions from local files to the Brockley server.
Creates resources that don't exist, updates resources that do (matched by name).

Files with both "base_url" and "endpoints" top-level keys are deployed as
API tool definitions. All other files are deployed as graphs.

Environment variable placeholders (${VAR_NAME}) in JSON files are
resolved from the current environment before pushing. Use --env-file to
load variables from a .env file.

Examples:
  brockley deploy -f graph.json
  brockley deploy -f api-tool.json
  brockley deploy -d definitions/ --env-file .env
  brockley deploy -f graph1.json -f api-tool1.json`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringSliceVarP(&deployFiles, "file", "f", nil, "Graph JSON file(s)")
	deployCmd.Flags().StringSliceVarP(&deployDirs, "dir", "d", nil, "Directory(ies) containing graph JSON files")
	deployCmd.Flags().StringVar(&deployNamespace, "namespace", "", "Override namespace for all graphs")
	deployCmd.Flags().StringVarP(&deployEnvFile, "env-file", "e", "", "Load environment variables from file (e.g. .env)")
	rootCmd.AddCommand(deployCmd)
}

// isAPIToolDefinition returns true if the parsed JSON has both "base_url"
// and "endpoints" top-level keys, indicating it is an API tool definition.
func isAPIToolDefinition(body map[string]any) bool {
	_, hasBaseURL := body["base_url"]
	_, hasEndpoints := body["endpoints"]
	return hasBaseURL && hasEndpoints
}

func runDeploy(cmd *cobra.Command, args []string) error {
	// Load env file if specified.
	if deployEnvFile != "" {
		if err := loadEnvFile(deployEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}
	}

	files, err := collectGraphFiles(deployFiles, deployDirs)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no files provided. Use -f or -d flags")
	}

	c := newClient()
	ctx := context.Background()

	// Fetch existing graphs to detect create vs update
	existingGraphs := make(map[string]string) // name → id
	graphResp, err := c.ListGraphs(ctx, "", "", "", 100)
	if err != nil {
		return fmt.Errorf("failed to list existing graphs: %w", err)
	}
	for _, item := range graphResp.Items {
		name := extractString(item, "name")
		id := extractString(item, "id")
		if name != "" && id != "" {
			existingGraphs[name] = id
		}
	}

	// Fetch existing API tools to detect create vs update
	existingAPITools := make(map[string]string) // name → id
	atResp, err := c.ListAPITools(ctx, "", "", 100)
	if err != nil {
		return fmt.Errorf("failed to list existing API tools: %w", err)
	}
	for _, item := range atResp.Items {
		name := extractString(item, "name")
		id := extractString(item, "id")
		if name != "" && id != "" {
			existingAPITools[name] = id
		}
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", f, err)
			continue
		}

		// Expand ${VAR} placeholders from environment variables.
		data, missing := expandEnvVars(data)
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "Warning: %s references unset variables: %s\n", f, strings.Join(missing, ", "))
		}

		var body map[string]any
		if err := json.Unmarshal(data, &body); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", f, err)
			continue
		}

		if deployNamespace != "" {
			body["namespace"] = deployNamespace
		}

		name, _ := body["name"].(string)
		if name == "" {
			fmt.Fprintf(os.Stderr, "Skipping %s: no 'name' field\n", f)
			continue
		}

		if isAPIToolDefinition(body) {
			deployAPITool(ctx, c, f, name, body, existingAPITools)
		} else {
			deployGraph(ctx, c, f, name, body, existingGraphs)
		}
	}

	return nil
}

func deployGraph(ctx context.Context, c *client.Client, file, name string, body map[string]any, existing map[string]string) {
	if id, found := existing[name]; found {
		// Update existing graph
		_, err := c.UpdateGraph(ctx, id, body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating graph %s (%s): %v\n", name, file, err)
			return
		}
		fmt.Printf("Updated graph: %s (id: %s)\n", name, id)
	} else {
		// Create new graph
		result, err := c.CreateGraph(ctx, body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating graph %s (%s): %v\n", name, file, err)
			return
		}
		newID := extractString(result, "id")
		fmt.Printf("Created graph: %s (id: %s)\n", name, newID)
	}
}

func deployAPITool(ctx context.Context, c *client.Client, file, name string, body map[string]any, existing map[string]string) {
	if id, found := existing[name]; found {
		// Update existing API tool
		_, err := c.UpdateAPITool(ctx, id, body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating API tool %s (%s): %v\n", name, file, err)
			return
		}
		fmt.Printf("Updated API tool: %s (id: %s)\n", name, id)
	} else {
		// Create new API tool
		result, err := c.CreateAPITool(ctx, body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating API tool %s (%s): %v\n", name, file, err)
			return
		}
		newID := extractString(result, "id")
		fmt.Printf("Created API tool: %s (id: %s)\n", name, newID)
	}
}
