package cmd

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// loadEnvFile reads a .env-style file and sets the variables in the process environment.
// Lines starting with # are comments. Empty lines are skipped.
// Format: KEY=VALUE or KEY="VALUE" or KEY='VALUE'.
// Does not overwrite variables already set in the environment.
func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open env file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := parseEnvLine(line)
		if !ok {
			return fmt.Errorf("env file %s:%d: invalid line: %s", path, lineNum, line)
		}

		// Don't overwrite existing env vars.
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// parseEnvLine parses a single KEY=VALUE line.
func parseEnvLine(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 1 {
		return "", "", false
	}

	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])

	// Strip surrounding quotes.
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	return key, value, true
}

// envVarPattern matches ${VAR_NAME} placeholders.
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnvVars replaces all ${VAR} placeholders in data with their values from os.Getenv.
// Use $${VAR} to emit a literal ${VAR} (the $$ escapes to a single $).
// Returns the expanded data and a list of variable names that were referenced but not set.
func expandEnvVars(data []byte) ([]byte, []string) {
	var missing []string
	seen := make(map[string]bool)

	// Step 1: Replace all $$ with a sentinel to protect escaped dollar signs.
	sentinel := "\x00DOLLAR\x00"
	escaped := strings.ReplaceAll(string(data), "$$", sentinel)

	// Step 2: Normal env var expansion on the remaining ${VAR} patterns.
	result := envVarPattern.ReplaceAllFunc([]byte(escaped), func(match []byte) []byte {
		name := string(match[2 : len(match)-1])
		value := os.Getenv(name)
		if value == "" && !seen[name] {
			missing = append(missing, name)
			seen[name] = true
		}
		return []byte(value)
	})

	// Step 3: Restore sentinels back to literal $.
	final := strings.ReplaceAll(string(result), sentinel, "$")

	return []byte(final), missing
}
