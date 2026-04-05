// Package secret provides SecretStore implementations for resolving
// api_key_ref names to actual secret values.
package secret

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// EnvSecretStore resolves api_key_ref to environment variables.
// Convention: ref "anthropic-primary" -> env var BROCKLEY_SECRET_ANTHROPIC_PRIMARY
type EnvSecretStore struct{}

// NewEnvSecretStore creates an EnvSecretStore.
func NewEnvSecretStore() *EnvSecretStore {
	return &EnvSecretStore{}
}

// GetSecret resolves a secret reference to its value from environment variables.
// The ref is transformed: uppercase, replace "-" with "_", prepend "BROCKLEY_SECRET_".
func (s *EnvSecretStore) GetSecret(_ context.Context, ref string) (string, error) {
	envKey := "BROCKLEY_SECRET_" + strings.ToUpper(strings.ReplaceAll(ref, "-", "_"))
	val := os.Getenv(envKey)
	if val == "" {
		return "", fmt.Errorf("secret %q not found: environment variable %s is not set", ref, envKey)
	}
	return val, nil
}
