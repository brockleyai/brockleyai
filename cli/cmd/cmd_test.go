package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractString(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		key      string
		expected string
	}{
		{"string field", `{"name": "test"}`, "name", "test"},
		{"number field", `{"version": 3}`, "version", "3"},
		{"missing field", `{"name": "test"}`, "missing", ""},
		{"empty object", `{}`, "name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractString(json.RawMessage(tt.data), tt.key)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeTerraformName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"my-graph", "my_graph"},
		{"my graph", "my_graph"},
		{"123start", "_123start"},
		{"CamelCase", "CamelCase"},
		{"with.dots", "with_dots"},
		{"", "graph"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeTerraformName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTerraformName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "(not set)"},
		{"short", "****"},
		{"abcdefghij", "abcd...ghij"},
		{"my-secret-api-key-123", "my-s...-123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := maskKey(tt.input)
			if result != tt.expected {
				t.Errorf("maskKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCollectGraphFiles(t *testing.T) {
	// Create temp dir with test files
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "graph1.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "graph2.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a graph"), 0644)

	files, err := collectGraphFiles(nil, []string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 json files, got %d", len(files))
	}
}

func TestCollectGraphFilesWithExplicitFiles(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "my-graph.json")
	os.WriteFile(f, []byte(`{}`), 0644)

	files, err := collectGraphFiles([]string{f}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set test env vars.
	t.Setenv("TEST_API_KEY", "sk-test-123")
	t.Setenv("TEST_MODEL", "gpt-4")

	input := []byte(`{"api_key": "${TEST_API_KEY}", "model": "${TEST_MODEL}", "other": "no-vars"}`)
	result, missing := expandEnvVars(input)

	expected := `{"api_key": "sk-test-123", "model": "gpt-4", "other": "no-vars"}`
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing vars, got %v", missing)
	}
}

func TestExpandEnvVars_Missing(t *testing.T) {
	input := []byte(`{"key": "${DEFINITELY_NOT_SET_12345}"}`)
	result, missing := expandEnvVars(input)

	expected := `{"key": ""}`
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
	if len(missing) != 1 || missing[0] != "DEFINITELY_NOT_SET_12345" {
		t.Errorf("expected [DEFINITELY_NOT_SET_12345], got %v", missing)
	}
}

func TestExpandEnvVars_NoPlaceholders(t *testing.T) {
	input := []byte(`{"key": "plain-value"}`)
	result, missing := expandEnvVars(input)

	if string(result) != string(input) {
		t.Errorf("expected unchanged output")
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing vars")
	}
}

func TestExpandEnvVars_EscapedDollar(t *testing.T) {
	t.Setenv("MY_VAR", "expanded_value")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escaped produces literal",
			input:    `$${MY_VAR}`,
			expected: `${MY_VAR}`,
		},
		{
			name:     "unescaped still expands",
			input:    `${MY_VAR}`,
			expected: `expanded_value`,
		},
		{
			name:     "mixed escaped and unescaped",
			input:    `prefix $${MY_VAR} middle ${MY_VAR} suffix`,
			expected: `prefix ${MY_VAR} middle expanded_value suffix`,
		},
		{
			name:     "triple dollar produces literal dollar plus expanded",
			input:    `$$${MY_VAR}`,
			expected: `$expanded_value`,
		},
		{
			name:     "multiple escaped in same string",
			input:    `$${A} and $${B}`,
			expected: `${A} and ${B}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := expandEnvVars([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("expandEnvVars(%q) = %q, want %q", tt.input, string(result), tt.expected)
			}
		})
	}
}

func TestParseEnvLine(t *testing.T) {
	tests := []struct {
		line      string
		wantKey   string
		wantValue string
		wantOk    bool
	}{
		{"KEY=value", "KEY", "value", true},
		{`KEY="quoted"`, "KEY", "quoted", true},
		{`KEY='single'`, "KEY", "single", true},
		{"KEY=", "KEY", "", true},
		{"=value", "", "", false},
		{"no-equals", "", "", false},
		{" KEY = value ", "KEY", "value", true},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			key, value, ok := parseEnvLine(tt.line)
			if ok != tt.wantOk {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOk)
			}
			if key != tt.wantKey {
				t.Errorf("key: got %q, want %q", key, tt.wantKey)
			}
			if value != tt.wantValue {
				t.Errorf("value: got %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := "MY_TEST_KEY_XYZ=hello-world\n# comment\n\nANOTHER_KEY=\"quoted value\"\n"
	os.WriteFile(envFile, []byte(content), 0644)

	// Clear these vars first.
	os.Unsetenv("MY_TEST_KEY_XYZ")
	os.Unsetenv("ANOTHER_KEY")

	err := loadEnvFile(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := os.Getenv("MY_TEST_KEY_XYZ"); got != "hello-world" {
		t.Errorf("MY_TEST_KEY_XYZ: got %q, want %q", got, "hello-world")
	}
	if got := os.Getenv("ANOTHER_KEY"); got != "quoted value" {
		t.Errorf("ANOTHER_KEY: got %q, want %q", got, "quoted value")
	}

	// Clean up.
	os.Unsetenv("MY_TEST_KEY_XYZ")
	os.Unsetenv("ANOTHER_KEY")
}

func TestLoadEnvFile_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("EXISTING_VAR=from-file\n"), 0644)

	t.Setenv("EXISTING_VAR", "from-env")

	err := loadEnvFile(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := os.Getenv("EXISTING_VAR"); got != "from-env" {
		t.Errorf("expected env to take priority, got %q", got)
	}
}

func TestExportToTerraform(t *testing.T) {
	graphJSON := json.RawMessage(`{
		"name": "test-graph",
		"namespace": "default",
		"description": "A test graph",
		"status": "active",
		"version": 1,
		"nodes": [
			{"id": "input-1", "name": "Input", "type": "input", "input_ports": [], "output_ports": [{"name": "text", "schema": {"type": "string"}}], "config": {}}
		],
		"edges": []
	}`)

	output, err := exportToTerraform(graphJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == "" {
		t.Fatal("expected non-empty terraform output")
	}

	// Check key parts
	if !contains(output, `resource "brockley_graph"`) {
		t.Error("expected resource block")
	}
	if !contains(output, `name        = "test-graph"`) {
		t.Error("expected name attribute")
	}
	if !contains(output, `namespace   = "default"`) {
		t.Error("expected namespace attribute")
	}
}

func TestExportToYAML(t *testing.T) {
	graphJSON := json.RawMessage(`{
		"name": "test-graph",
		"namespace": "default",
		"status": "active",
		"version": 1,
		"nodes": [
			{"id": "input-1", "name": "Input", "type": "input", "input_ports": [], "output_ports": [{"name": "text", "schema": {"type": "string"}}], "config": {}}
		],
		"edges": []
	}`)

	output, err := exportToYAML(graphJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(output, "name: test-graph") {
		t.Error("expected name field in YAML output")
	}
	if !contains(output, "namespace: default") {
		t.Error("expected namespace field in YAML output")
	}
}

func TestValidateLocal(t *testing.T) {
	// Create a valid graph file
	dir := t.TempDir()
	graphJSON := `{
		"name": "test",
		"namespace": "default",
		"nodes": [
			{"id": "input-1", "name": "Input", "type": "input", "input_ports": [{"name": "text", "schema": {"type": "string"}}], "output_ports": [{"name": "text", "schema": {"type": "string"}}], "config": {}}
		],
		"edges": []
	}`
	f := filepath.Join(dir, "graph.json")
	os.WriteFile(f, []byte(graphJSON), 0644)

	// Set up flags for local validation
	validateFiles = []string{f}
	validateDirs = nil
	flagOutput = "json"

	// Should not error
	err := validateLocal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
