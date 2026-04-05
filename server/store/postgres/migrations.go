package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationSQL = `
CREATE TABLE IF NOT EXISTS graphs (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    namespace TEXT NOT NULL DEFAULT 'default',
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft',
    graph_data JSONB NOT NULL DEFAULT '{}',
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(tenant_id, namespace, name, version)
);

CREATE TABLE IF NOT EXISTS schemas (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT 'default',
    description TEXT DEFAULT '',
    json_schema JSONB NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(tenant_id, namespace, name)
);

CREATE TABLE IF NOT EXISTS prompt_templates (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT 'default',
    description TEXT DEFAULT '',
    template_data JSONB NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(tenant_id, namespace, name)
);

CREATE TABLE IF NOT EXISTS provider_configs (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT 'default',
    provider TEXT NOT NULL,
    base_url TEXT DEFAULT '',
    api_key_ref TEXT NOT NULL,
    default_model TEXT DEFAULT '',
    extra_headers JSONB,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(tenant_id, namespace, name)
);

CREATE TABLE IF NOT EXISTS executions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    graph_id TEXT NOT NULL,
    graph_version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    input JSONB NOT NULL,
    output JSONB,
    state JSONB,
    error JSONB,
    iteration_counts JSONB,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    timeout_seconds INTEGER,
    trigger TEXT NOT NULL DEFAULT 'api',
    correlation_id TEXT DEFAULT '',
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS execution_steps (
    id TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL REFERENCES executions(id),
    node_id TEXT NOT NULL,
    node_type TEXT NOT NULL,
    iteration INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    input JSONB,
    output JSONB,
    state_before JSONB,
    state_after JSONB,
    error JSONB,
    attempt INTEGER NOT NULL DEFAULT 1,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration_ms BIGINT,
    llm_usage JSONB,
    llm_debug JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(execution_id, node_id, iteration, attempt)
);

ALTER TABLE execution_steps ADD COLUMN IF NOT EXISTS llm_debug JSONB;

CREATE INDEX IF NOT EXISTS idx_graphs_tenant_ns_status ON graphs(tenant_id, namespace, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_schemas_tenant_ns ON schemas(tenant_id, namespace) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_prompt_templates_tenant_ns ON prompt_templates(tenant_id, namespace) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_provider_configs_tenant_ns ON provider_configs(tenant_id, namespace) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_executions_tenant_graph ON executions(tenant_id, graph_id, created_at);
CREATE INDEX IF NOT EXISTS idx_executions_tenant_status ON executions(tenant_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_executions_correlation ON executions(tenant_id, correlation_id) WHERE correlation_id != '';
CREATE INDEX IF NOT EXISTS idx_execution_steps_exec ON execution_steps(execution_id, created_at);

CREATE TABLE IF NOT EXISTS api_tool_definitions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT 'default',
    description TEXT DEFAULT '',
    base_url TEXT NOT NULL,
    definition JSONB NOT NULL DEFAULT '{}',
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(tenant_id, namespace, name)
);

CREATE INDEX IF NOT EXISTS idx_api_tool_definitions_tenant_ns ON api_tool_definitions(tenant_id, namespace) WHERE deleted_at IS NULL;
`

// RunMigrations executes the initial schema migration against the database.
// It uses a PostgreSQL advisory lock to prevent concurrent migration runs
// (e.g., when server and worker start simultaneously).
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	// Advisory lock key — arbitrary constant unique to migrations.
	const lockID int64 = 8427101 // arbitrary constant for migration advisory lock
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		return fmt.Errorf("acquiring migration lock: %w", err)
	}
	defer conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID) //nolint:errcheck

	if _, err := conn.Exec(ctx, migrationSQL); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}
