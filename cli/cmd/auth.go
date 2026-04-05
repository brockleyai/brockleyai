package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication configuration",
}

var authSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Save server URL and API key to config file",
	RunE:  runAuthSet,
}

var authShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current auth configuration",
	RunE:  runAuthShow,
}

var authTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connectivity to the server",
	RunE:  runAuthTest,
}

var (
	authServer string
	authKey    string
)

func init() {
	authSetCmd.Flags().StringVar(&authServer, "server", "", "Server URL")
	authSetCmd.Flags().StringVar(&authKey, "key", "", "API key")
	authCmd.AddCommand(authSetCmd, authShowCmd, authTestCmd)
	rootCmd.AddCommand(authCmd)
}

// AuthConfig stores CLI auth settings.
type AuthConfig struct {
	ServerURL string `json:"server_url"`
	APIKey    string `json:"api_key,omitempty"`
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".brockley"
	}
	return filepath.Join(home, ".brockley")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func loadAuthConfig() (*AuthConfig, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &AuthConfig{}, nil
		}
		return nil, err
	}
	var cfg AuthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveAuthConfig(cfg *AuthConfig) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

func runAuthSet(cmd *cobra.Command, args []string) error {
	cfg, err := loadAuthConfig()
	if err != nil {
		cfg = &AuthConfig{}
	}

	if authServer != "" {
		cfg.ServerURL = authServer
	}
	if authKey != "" {
		cfg.APIKey = authKey
	}

	if err := saveAuthConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Config saved to %s\n", configPath())
	return nil
}

func runAuthShow(cmd *cobra.Command, args []string) error {
	cfg, err := loadAuthConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Config file: %s\n", configPath())
	fmt.Printf("Server URL:  %s\n", effectiveServerURL(cfg))
	fmt.Printf("API Key:     %s\n", maskKey(effectiveAPIKey(cfg)))
	return nil
}

func runAuthTest(cmd *cobra.Command, args []string) error {
	c := newClient()
	result, err := c.Health(context.Background())
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	fmt.Printf("Connected to %s\n", flagServerURL)
	if flagOutput == "json" {
		return printRawJSON(result)
	}

	fmt.Printf("  Status: %s\n", extractString(result, "status"))
	return nil
}

func effectiveServerURL(cfg *AuthConfig) string {
	if v := os.Getenv("BROCKLEY_SERVER_URL"); v != "" {
		return v + " (from BROCKLEY_SERVER_URL)"
	}
	if cfg.ServerURL != "" {
		return cfg.ServerURL + " (from config file)"
	}
	return "http://localhost:8000 (default)"
}

func effectiveAPIKey(cfg *AuthConfig) string {
	if v := os.Getenv("BROCKLEY_API_KEY"); v != "" {
		return v
	}
	return cfg.APIKey
}

func maskKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
