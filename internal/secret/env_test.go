package secret

import (
	"context"
	"os"
	"testing"
)

func TestEnvSecretStore_GetSecret(t *testing.T) {
	store := NewEnvSecretStore()
	ctx := context.Background()

	t.Run("returns value when env var is set", func(t *testing.T) {
		os.Setenv("BROCKLEY_SECRET_ANTHROPIC_PRIMARY", "sk-test-123")
		defer os.Unsetenv("BROCKLEY_SECRET_ANTHROPIC_PRIMARY")

		val, err := store.GetSecret(ctx, "anthropic-primary")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "sk-test-123" {
			t.Errorf("got %q, want %q", val, "sk-test-123")
		}
	})

	t.Run("returns error when env var is not set", func(t *testing.T) {
		os.Unsetenv("BROCKLEY_SECRET_OPENAI_MAIN")

		_, err := store.GetSecret(ctx, "openai-main")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := err.Error(); got == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("transforms ref name correctly", func(t *testing.T) {
		os.Setenv("BROCKLEY_SECRET_MY_CUSTOM_KEY", "secret-value")
		defer os.Unsetenv("BROCKLEY_SECRET_MY_CUSTOM_KEY")

		val, err := store.GetSecret(ctx, "my-custom-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "secret-value" {
			t.Errorf("got %q, want %q", val, "secret-value")
		}
	})

	t.Run("handles ref with underscores", func(t *testing.T) {
		os.Setenv("BROCKLEY_SECRET_ALREADY_UNDERSCORE", "val")
		defer os.Unsetenv("BROCKLEY_SECRET_ALREADY_UNDERSCORE")

		val, err := store.GetSecret(ctx, "already_underscore")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "val" {
			t.Errorf("got %q, want %q", val, "val")
		}
	})

	t.Run("handles mixed case ref", func(t *testing.T) {
		os.Setenv("BROCKLEY_SECRET_MIXEDCASE", "val2")
		defer os.Unsetenv("BROCKLEY_SECRET_MIXEDCASE")

		val, err := store.GetSecret(ctx, "MixedCase")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "val2" {
			t.Errorf("got %q, want %q", val, "val2")
		}
	})
}
