package trace

import "os"

// SetupFromEnv reads BROCKLEY_TRACE_* environment variables and creates
// an ExporterRegistry with all enabled exporters registered.
// If no exporters are configured, returns an empty (but valid) registry.
func SetupFromEnv() *ExporterRegistry {
	registry := NewExporterRegistry()

	// Langfuse
	if os.Getenv("BROCKLEY_TRACE_LANGFUSE_ENABLED") == "true" {
		host := os.Getenv("BROCKLEY_TRACE_LANGFUSE_HOST")
		publicKey := os.Getenv("BROCKLEY_TRACE_LANGFUSE_PUBLIC_KEY")
		secretKey := os.Getenv("BROCKLEY_TRACE_LANGFUSE_SECRET_KEY")
		if host != "" && publicKey != "" && secretKey != "" {
			registry.Register(NewLangfuseExporter(host, publicKey, secretKey))
		}
	}

	// Opik
	if os.Getenv("BROCKLEY_TRACE_OPIK_ENABLED") == "true" {
		host := os.Getenv("BROCKLEY_TRACE_OPIK_HOST")
		apiKey := os.Getenv("BROCKLEY_TRACE_OPIK_API_KEY")
		workspace := os.Getenv("BROCKLEY_TRACE_OPIK_WORKSPACE")
		if host != "" && apiKey != "" {
			if workspace == "" {
				workspace = "default"
			}
			registry.Register(NewOpikExporter(host, apiKey, workspace))
		}
	}

	// Arize Phoenix
	if os.Getenv("BROCKLEY_TRACE_PHOENIX_ENABLED") == "true" {
		host := os.Getenv("BROCKLEY_TRACE_PHOENIX_HOST")
		apiKey := os.Getenv("BROCKLEY_TRACE_PHOENIX_API_KEY") // optional
		if host != "" {
			registry.Register(NewPhoenixExporter(host, apiKey))
		}
	}

	// LangSmith
	if os.Getenv("BROCKLEY_TRACE_LANGSMITH_ENABLED") == "true" {
		apiKey := os.Getenv("BROCKLEY_TRACE_LANGSMITH_API_KEY")
		project := os.Getenv("BROCKLEY_TRACE_LANGSMITH_PROJECT")
		if apiKey != "" {
			if project == "" {
				project = "brockley-traces"
			}
			registry.Register(NewLangSmithExporter(apiKey, project))
		}
	}

	// Generic OTLP
	if os.Getenv("BROCKLEY_TRACE_OTLP_ENABLED") == "true" {
		endpoint := os.Getenv("BROCKLEY_TRACE_OTLP_ENDPOINT")
		if endpoint != "" {
			headers := parseHeaders(os.Getenv("BROCKLEY_TRACE_OTLP_HEADERS"))
			registry.Register(NewOTLPExporter(endpoint, headers))
		}
	}

	return registry
}

// parseHeaders parses a comma-separated list of key=value pairs into a map.
// Example: "Authorization=Bearer token123,X-Custom=value"
func parseHeaders(raw string) map[string]string {
	headers := make(map[string]string)
	if raw == "" {
		return headers
	}
	for _, pair := range splitHeaders(raw) {
		if idx := indexOf(pair, '='); idx > 0 {
			headers[pair[:idx]] = pair[idx+1:]
		}
	}
	return headers
}

// splitHeaders splits on commas, but is simple since header values
// should not contain commas in typical OTLP config.
func splitHeaders(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			part := s[start:i]
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
