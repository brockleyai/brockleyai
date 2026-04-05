package mock

import (
	"context"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// MockSecretStore is an in-memory test double for model.SecretStore.
type MockSecretStore struct {
	// Secrets maps ref names to secret values.
	Secrets map[string]string
}

var _ model.SecretStore = (*MockSecretStore)(nil)

// NewMockSecretStore creates a MockSecretStore with an initialized map.
func NewMockSecretStore() *MockSecretStore {
	return &MockSecretStore{
		Secrets: make(map[string]string),
	}
}

func (m *MockSecretStore) GetSecret(ctx context.Context, ref string) (string, error) {
	val, ok := m.Secrets[ref]
	if !ok {
		return "", fmt.Errorf("secret %q not found", ref)
	}
	return val, nil
}
