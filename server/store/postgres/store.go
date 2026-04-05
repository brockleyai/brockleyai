// Package postgres implements the model.Store interface backed by PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brockleyai/brockleyai/internal/model"
)

// PostgresStore implements model.Store using PostgreSQL with pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// Compile-time check that PostgresStore implements model.Store.
var _ model.Store = (*PostgresStore)(nil)

// New creates a new PostgresStore, connects to the database, and runs migrations.
func New(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	if err := RunMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Close closes the underlying connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// CheckHealth pings the database to verify connectivity.
func (s *PostgresStore) CheckHealth(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// graphData is the JSONB structure stored in the graph_data column.
type graphData struct {
	Nodes []model.Node      `json:"nodes"`
	Edges []model.Edge      `json:"edges"`
	State *model.GraphState `json:"state,omitempty"`
}

// --- Graphs ---

func (s *PostgresStore) CreateGraph(ctx context.Context, graph *model.Graph) error {
	gd := graphData{Nodes: graph.Nodes, Edges: graph.Edges, State: graph.State}
	gdJSON, err := json.Marshal(gd)
	if err != nil {
		return fmt.Errorf("marshaling graph_data: %w", err)
	}
	now := time.Now().UTC()
	graph.CreatedAt = now
	graph.UpdatedAt = now

	_, err = s.pool.Exec(ctx, `
		INSERT INTO graphs (id, tenant_id, name, description, namespace, version, status, graph_data, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		graph.ID, graph.TenantID, graph.Name, graph.Description,
		graph.Namespace, graph.Version, string(graph.Status),
		gdJSON, nullableJSON(graph.Metadata),
		graph.CreatedAt, graph.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting graph: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetGraph(ctx context.Context, tenantID, id string) (*model.Graph, error) {
	g := &model.Graph{}
	var gdJSON []byte
	var metadata []byte
	var status string
	var deletedAt *time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, namespace, version, status, graph_data, metadata,
		       created_at, updated_at, deleted_at
		FROM graphs
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID,
	).Scan(
		&g.ID, &g.TenantID, &g.Name, &g.Description, &g.Namespace, &g.Version,
		&status, &gdJSON, &metadata, &g.CreatedAt, &g.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("graph not found: %s", id)
		}
		return nil, fmt.Errorf("querying graph: %w", err)
	}
	g.Status = model.GraphStatus(status)
	g.DeletedAt = deletedAt
	if metadata != nil {
		g.Metadata = json.RawMessage(metadata)
	}

	var gd graphData
	if err := json.Unmarshal(gdJSON, &gd); err != nil {
		return nil, fmt.Errorf("unmarshaling graph_data: %w", err)
	}
	g.Nodes = gd.Nodes
	g.Edges = gd.Edges
	g.State = gd.State
	return g, nil
}

func (s *PostgresStore) ListGraphs(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.Graph, string, error) {
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	var rows pgx.Rows
	var err error
	if namespace != "" {
		if cursor == "" {
			rows, err = s.pool.Query(ctx, `
				SELECT id, tenant_id, name, description, namespace, version, status, graph_data, metadata,
				       created_at, updated_at, deleted_at
				FROM graphs
				WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL
				ORDER BY id
				LIMIT $3`, tenantID, namespace, fetchLimit)
		} else {
			rows, err = s.pool.Query(ctx, `
				SELECT id, tenant_id, name, description, namespace, version, status, graph_data, metadata,
				       created_at, updated_at, deleted_at
				FROM graphs
				WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL AND id > $3
				ORDER BY id
				LIMIT $4`, tenantID, namespace, cursor, fetchLimit)
		}
	} else {
		if cursor == "" {
			rows, err = s.pool.Query(ctx, `
				SELECT id, tenant_id, name, description, namespace, version, status, graph_data, metadata,
				       created_at, updated_at, deleted_at
				FROM graphs
				WHERE tenant_id = $1 AND deleted_at IS NULL
				ORDER BY id
				LIMIT $2`, tenantID, fetchLimit)
		} else {
			rows, err = s.pool.Query(ctx, `
				SELECT id, tenant_id, name, description, namespace, version, status, graph_data, metadata,
				       created_at, updated_at, deleted_at
				FROM graphs
				WHERE tenant_id = $1 AND deleted_at IS NULL AND id > $2
				ORDER BY id
				LIMIT $3`, tenantID, cursor, fetchLimit)
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing graphs: %w", err)
	}
	defer rows.Close()

	var graphs []*model.Graph
	for rows.Next() {
		g := &model.Graph{}
		var gdJSON, metadata []byte
		var status string
		var deletedAt *time.Time
		if err := rows.Scan(
			&g.ID, &g.TenantID, &g.Name, &g.Description, &g.Namespace, &g.Version,
			&status, &gdJSON, &metadata, &g.CreatedAt, &g.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scanning graph row: %w", err)
		}
		g.Status = model.GraphStatus(status)
		g.DeletedAt = deletedAt
		if metadata != nil {
			g.Metadata = json.RawMessage(metadata)
		}
		var gd graphData
		if err := json.Unmarshal(gdJSON, &gd); err != nil {
			return nil, "", fmt.Errorf("unmarshaling graph_data: %w", err)
		}
		g.Nodes = gd.Nodes
		g.Edges = gd.Edges
		g.State = gd.State
		graphs = append(graphs, g)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterating graph rows: %w", err)
	}

	nextCursor := ""
	if len(graphs) > limit {
		nextCursor = graphs[limit-1].ID
		graphs = graphs[:limit]
	}
	return graphs, nextCursor, nil
}

func (s *PostgresStore) UpdateGraph(ctx context.Context, graph *model.Graph) error {
	gd := graphData{Nodes: graph.Nodes, Edges: graph.Edges, State: graph.State}
	gdJSON, err := json.Marshal(gd)
	if err != nil {
		return fmt.Errorf("marshaling graph_data: %w", err)
	}
	graph.UpdatedAt = time.Now().UTC()

	tag, err := s.pool.Exec(ctx, `
		UPDATE graphs
		SET name = $1, description = $2, namespace = $3, version = $4, status = $5,
		    graph_data = $6, metadata = $7, updated_at = $8
		WHERE id = $9 AND tenant_id = $10 AND deleted_at IS NULL`,
		graph.Name, graph.Description, graph.Namespace, graph.Version, string(graph.Status),
		gdJSON, nullableJSON(graph.Metadata), graph.UpdatedAt,
		graph.ID, graph.TenantID,
	)
	if err != nil {
		return fmt.Errorf("updating graph: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("graph not found: %s", graph.ID)
	}
	return nil
}

func (s *PostgresStore) DeleteGraph(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE graphs SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID)
	if err != nil {
		return fmt.Errorf("soft-deleting graph: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("graph not found: %s", id)
	}
	return nil
}

// --- Schemas ---

func (s *PostgresStore) CreateSchema(ctx context.Context, schema *model.SchemaLibrary) error {
	now := time.Now().UTC()
	schema.CreatedAt = now
	schema.UpdatedAt = now
	_, err := s.pool.Exec(ctx, `
		INSERT INTO schemas (id, tenant_id, name, namespace, description, json_schema, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		schema.ID, schema.TenantID, schema.Name, schema.Namespace, schema.Description,
		[]byte(schema.JSONSchema), nullableJSON(schema.Metadata),
		schema.CreatedAt, schema.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting schema: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetSchema(ctx context.Context, tenantID, id string) (*model.SchemaLibrary, error) {
	sc := &model.SchemaLibrary{}
	var jsonSchema, metadata []byte
	var deletedAt *time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, namespace, description, json_schema, metadata, created_at, updated_at, deleted_at
		FROM schemas
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID,
	).Scan(
		&sc.ID, &sc.TenantID, &sc.Name, &sc.Namespace, &sc.Description,
		&jsonSchema, &metadata, &sc.CreatedAt, &sc.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("schema not found: %s", id)
		}
		return nil, fmt.Errorf("querying schema: %w", err)
	}
	sc.JSONSchema = json.RawMessage(jsonSchema)
	sc.DeletedAt = deletedAt
	if metadata != nil {
		sc.Metadata = json.RawMessage(metadata)
	}
	return sc, nil
}

func (s *PostgresStore) ListSchemas(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.SchemaLibrary, string, error) {
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, namespace, description, json_schema, metadata, created_at, updated_at, deleted_at
			FROM schemas
			WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL
			ORDER BY id LIMIT $3`, tenantID, namespace, fetchLimit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, namespace, description, json_schema, metadata, created_at, updated_at, deleted_at
			FROM schemas
			WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL AND id > $3
			ORDER BY id LIMIT $4`, tenantID, namespace, cursor, fetchLimit)
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing schemas: %w", err)
	}
	defer rows.Close()

	var schemas []*model.SchemaLibrary
	for rows.Next() {
		sc := &model.SchemaLibrary{}
		var jsonSchema, metadata []byte
		var deletedAt *time.Time
		if err := rows.Scan(
			&sc.ID, &sc.TenantID, &sc.Name, &sc.Namespace, &sc.Description,
			&jsonSchema, &metadata, &sc.CreatedAt, &sc.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scanning schema row: %w", err)
		}
		sc.JSONSchema = json.RawMessage(jsonSchema)
		sc.DeletedAt = deletedAt
		if metadata != nil {
			sc.Metadata = json.RawMessage(metadata)
		}
		schemas = append(schemas, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterating schema rows: %w", err)
	}

	nextCursor := ""
	if len(schemas) > limit {
		nextCursor = schemas[limit-1].ID
		schemas = schemas[:limit]
	}
	return schemas, nextCursor, nil
}

func (s *PostgresStore) UpdateSchema(ctx context.Context, schema *model.SchemaLibrary) error {
	schema.UpdatedAt = time.Now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE schemas
		SET name = $1, namespace = $2, description = $3, json_schema = $4, metadata = $5, updated_at = $6
		WHERE id = $7 AND tenant_id = $8 AND deleted_at IS NULL`,
		schema.Name, schema.Namespace, schema.Description,
		[]byte(schema.JSONSchema), nullableJSON(schema.Metadata), schema.UpdatedAt,
		schema.ID, schema.TenantID,
	)
	if err != nil {
		return fmt.Errorf("updating schema: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("schema not found: %s", schema.ID)
	}
	return nil
}

func (s *PostgresStore) DeleteSchema(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE schemas SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID)
	if err != nil {
		return fmt.Errorf("soft-deleting schema: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("schema not found: %s", id)
	}
	return nil
}

// --- Prompt Templates ---

// templateData is the JSONB structure for the template_data column.
type templateData struct {
	SystemPrompt   string               `json:"system_prompt,omitempty"`
	UserPrompt     string               `json:"user_prompt"`
	Variables      []model.TemplateVar  `json:"variables"`
	ResponseFormat model.ResponseFormat `json:"response_format,omitempty"`
	OutputSchema   json.RawMessage      `json:"output_schema,omitempty"`
}

func (s *PostgresStore) CreatePromptTemplate(ctx context.Context, pt *model.PromptLibrary) error {
	td := templateData{
		SystemPrompt:   pt.SystemPrompt,
		UserPrompt:     pt.UserPrompt,
		Variables:      pt.Variables,
		ResponseFormat: pt.ResponseFormat,
		OutputSchema:   pt.OutputSchema,
	}
	tdJSON, err := json.Marshal(td)
	if err != nil {
		return fmt.Errorf("marshaling template_data: %w", err)
	}
	now := time.Now().UTC()
	pt.CreatedAt = now
	pt.UpdatedAt = now

	_, err = s.pool.Exec(ctx, `
		INSERT INTO prompt_templates (id, tenant_id, name, namespace, description, template_data, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		pt.ID, pt.TenantID, pt.Name, pt.Namespace, pt.Description,
		tdJSON, nullableJSON(pt.Metadata),
		pt.CreatedAt, pt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting prompt template: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetPromptTemplate(ctx context.Context, tenantID, id string) (*model.PromptLibrary, error) {
	pt := &model.PromptLibrary{}
	var tdJSON, metadata []byte
	var deletedAt *time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, namespace, description, template_data, metadata, created_at, updated_at, deleted_at
		FROM prompt_templates
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID,
	).Scan(
		&pt.ID, &pt.TenantID, &pt.Name, &pt.Namespace, &pt.Description,
		&tdJSON, &metadata, &pt.CreatedAt, &pt.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("prompt template not found: %s", id)
		}
		return nil, fmt.Errorf("querying prompt template: %w", err)
	}
	pt.DeletedAt = deletedAt
	if metadata != nil {
		pt.Metadata = json.RawMessage(metadata)
	}
	var td templateData
	if err := json.Unmarshal(tdJSON, &td); err != nil {
		return nil, fmt.Errorf("unmarshaling template_data: %w", err)
	}
	pt.SystemPrompt = td.SystemPrompt
	pt.UserPrompt = td.UserPrompt
	pt.Variables = td.Variables
	pt.ResponseFormat = td.ResponseFormat
	pt.OutputSchema = td.OutputSchema
	return pt, nil
}

func (s *PostgresStore) ListPromptTemplates(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.PromptLibrary, string, error) {
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, namespace, description, template_data, metadata, created_at, updated_at, deleted_at
			FROM prompt_templates
			WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL
			ORDER BY id LIMIT $3`, tenantID, namespace, fetchLimit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, namespace, description, template_data, metadata, created_at, updated_at, deleted_at
			FROM prompt_templates
			WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL AND id > $3
			ORDER BY id LIMIT $4`, tenantID, namespace, cursor, fetchLimit)
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing prompt templates: %w", err)
	}
	defer rows.Close()

	var templates []*model.PromptLibrary
	for rows.Next() {
		pt := &model.PromptLibrary{}
		var tdJSON, metadata []byte
		var deletedAt *time.Time
		if err := rows.Scan(
			&pt.ID, &pt.TenantID, &pt.Name, &pt.Namespace, &pt.Description,
			&tdJSON, &metadata, &pt.CreatedAt, &pt.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scanning prompt template row: %w", err)
		}
		pt.DeletedAt = deletedAt
		if metadata != nil {
			pt.Metadata = json.RawMessage(metadata)
		}
		var td templateData
		if err := json.Unmarshal(tdJSON, &td); err != nil {
			return nil, "", fmt.Errorf("unmarshaling template_data: %w", err)
		}
		pt.SystemPrompt = td.SystemPrompt
		pt.UserPrompt = td.UserPrompt
		pt.Variables = td.Variables
		pt.ResponseFormat = td.ResponseFormat
		pt.OutputSchema = td.OutputSchema
		templates = append(templates, pt)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterating prompt template rows: %w", err)
	}

	nextCursor := ""
	if len(templates) > limit {
		nextCursor = templates[limit-1].ID
		templates = templates[:limit]
	}
	return templates, nextCursor, nil
}

func (s *PostgresStore) UpdatePromptTemplate(ctx context.Context, pt *model.PromptLibrary) error {
	td := templateData{
		SystemPrompt:   pt.SystemPrompt,
		UserPrompt:     pt.UserPrompt,
		Variables:      pt.Variables,
		ResponseFormat: pt.ResponseFormat,
		OutputSchema:   pt.OutputSchema,
	}
	tdJSON, err := json.Marshal(td)
	if err != nil {
		return fmt.Errorf("marshaling template_data: %w", err)
	}
	pt.UpdatedAt = time.Now().UTC()

	tag, err := s.pool.Exec(ctx, `
		UPDATE prompt_templates
		SET name = $1, namespace = $2, description = $3, template_data = $4, metadata = $5, updated_at = $6
		WHERE id = $7 AND tenant_id = $8 AND deleted_at IS NULL`,
		pt.Name, pt.Namespace, pt.Description,
		tdJSON, nullableJSON(pt.Metadata), pt.UpdatedAt,
		pt.ID, pt.TenantID,
	)
	if err != nil {
		return fmt.Errorf("updating prompt template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("prompt template not found: %s", pt.ID)
	}
	return nil
}

func (s *PostgresStore) DeletePromptTemplate(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE prompt_templates SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID)
	if err != nil {
		return fmt.Errorf("soft-deleting prompt template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("prompt template not found: %s", id)
	}
	return nil
}

// --- Provider Configs ---

func (s *PostgresStore) CreateProviderConfig(ctx context.Context, pc *model.ProviderConfigLibrary) error {
	extraHeadersJSON, err := json.Marshal(pc.ExtraHeaders)
	if err != nil {
		return fmt.Errorf("marshaling extra_headers: %w", err)
	}
	now := time.Now().UTC()
	pc.CreatedAt = now
	pc.UpdatedAt = now

	_, err = s.pool.Exec(ctx, `
		INSERT INTO provider_configs (id, tenant_id, name, namespace, provider, base_url, api_key_ref, default_model, extra_headers, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		pc.ID, pc.TenantID, pc.Name, pc.Namespace, string(pc.Provider),
		pc.BaseURL, pc.APIKeyRef, pc.DefaultModel,
		extraHeadersJSON, nullableJSON(pc.Metadata),
		pc.CreatedAt, pc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting provider config: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetProviderConfig(ctx context.Context, tenantID, id string) (*model.ProviderConfigLibrary, error) {
	pc := &model.ProviderConfigLibrary{}
	var provider string
	var extraHeadersJSON, metadata []byte
	var deletedAt *time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, namespace, provider, base_url, api_key_ref, default_model,
		       extra_headers, metadata, created_at, updated_at, deleted_at
		FROM provider_configs
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID,
	).Scan(
		&pc.ID, &pc.TenantID, &pc.Name, &pc.Namespace, &provider,
		&pc.BaseURL, &pc.APIKeyRef, &pc.DefaultModel,
		&extraHeadersJSON, &metadata, &pc.CreatedAt, &pc.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("provider config not found: %s", id)
		}
		return nil, fmt.Errorf("querying provider config: %w", err)
	}
	pc.Provider = model.ProviderType(provider)
	pc.DeletedAt = deletedAt
	if metadata != nil {
		pc.Metadata = json.RawMessage(metadata)
	}
	if extraHeadersJSON != nil {
		if err := json.Unmarshal(extraHeadersJSON, &pc.ExtraHeaders); err != nil {
			return nil, fmt.Errorf("unmarshaling extra_headers: %w", err)
		}
	}
	return pc, nil
}

func (s *PostgresStore) ListProviderConfigs(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.ProviderConfigLibrary, string, error) {
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, namespace, provider, base_url, api_key_ref, default_model,
			       extra_headers, metadata, created_at, updated_at, deleted_at
			FROM provider_configs
			WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL
			ORDER BY id LIMIT $3`, tenantID, namespace, fetchLimit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, namespace, provider, base_url, api_key_ref, default_model,
			       extra_headers, metadata, created_at, updated_at, deleted_at
			FROM provider_configs
			WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL AND id > $3
			ORDER BY id LIMIT $4`, tenantID, namespace, cursor, fetchLimit)
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing provider configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.ProviderConfigLibrary
	for rows.Next() {
		pc := &model.ProviderConfigLibrary{}
		var provider string
		var extraHeadersJSON, metadata []byte
		var deletedAt *time.Time
		if err := rows.Scan(
			&pc.ID, &pc.TenantID, &pc.Name, &pc.Namespace, &provider,
			&pc.BaseURL, &pc.APIKeyRef, &pc.DefaultModel,
			&extraHeadersJSON, &metadata, &pc.CreatedAt, &pc.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scanning provider config row: %w", err)
		}
		pc.Provider = model.ProviderType(provider)
		pc.DeletedAt = deletedAt
		if metadata != nil {
			pc.Metadata = json.RawMessage(metadata)
		}
		if extraHeadersJSON != nil {
			if err := json.Unmarshal(extraHeadersJSON, &pc.ExtraHeaders); err != nil {
				return nil, "", fmt.Errorf("unmarshaling extra_headers: %w", err)
			}
		}
		configs = append(configs, pc)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterating provider config rows: %w", err)
	}

	nextCursor := ""
	if len(configs) > limit {
		nextCursor = configs[limit-1].ID
		configs = configs[:limit]
	}
	return configs, nextCursor, nil
}

func (s *PostgresStore) UpdateProviderConfig(ctx context.Context, pc *model.ProviderConfigLibrary) error {
	extraHeadersJSON, err := json.Marshal(pc.ExtraHeaders)
	if err != nil {
		return fmt.Errorf("marshaling extra_headers: %w", err)
	}
	pc.UpdatedAt = time.Now().UTC()

	tag, err := s.pool.Exec(ctx, `
		UPDATE provider_configs
		SET name = $1, namespace = $2, provider = $3, base_url = $4, api_key_ref = $5,
		    default_model = $6, extra_headers = $7, metadata = $8, updated_at = $9
		WHERE id = $10 AND tenant_id = $11 AND deleted_at IS NULL`,
		pc.Name, pc.Namespace, string(pc.Provider), pc.BaseURL, pc.APIKeyRef,
		pc.DefaultModel, extraHeadersJSON, nullableJSON(pc.Metadata), pc.UpdatedAt,
		pc.ID, pc.TenantID,
	)
	if err != nil {
		return fmt.Errorf("updating provider config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("provider config not found: %s", pc.ID)
	}
	return nil
}

func (s *PostgresStore) DeleteProviderConfig(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE provider_configs SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID)
	if err != nil {
		return fmt.Errorf("soft-deleting provider config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("provider config not found: %s", id)
	}
	return nil
}

// --- API Tool Definitions ---

// apiToolDefData is the JSONB structure stored in the definition column.
type apiToolDefData struct {
	DefaultHeaders []model.HeaderConfig `json:"default_headers,omitempty"`
	DefaultTimeout int                  `json:"default_timeout_ms,omitempty"`
	Retry          *model.RetryConfig   `json:"retry,omitempty"`
	Endpoints      []model.APIEndpoint  `json:"endpoints"`
}

func (s *PostgresStore) CreateAPITool(ctx context.Context, at *model.APIToolDefinition) error {
	defData := apiToolDefData{
		DefaultHeaders: at.DefaultHeaders,
		DefaultTimeout: at.DefaultTimeout,
		Retry:          at.Retry,
		Endpoints:      at.Endpoints,
	}
	defJSON, err := json.Marshal(defData)
	if err != nil {
		return fmt.Errorf("marshaling definition: %w", err)
	}
	now := time.Now().UTC()
	at.CreatedAt = now
	at.UpdatedAt = now

	_, err = s.pool.Exec(ctx, `
		INSERT INTO api_tool_definitions (id, tenant_id, name, namespace, description, base_url, definition, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		at.ID, at.TenantID, at.Name, at.Namespace, at.Description,
		at.BaseURL, defJSON, nullableJSON(at.Metadata),
		at.CreatedAt, at.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting api tool definition: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetAPITool(ctx context.Context, tenantID, id string) (*model.APIToolDefinition, error) {
	at := &model.APIToolDefinition{}
	var defJSON, metadata []byte
	var deletedAt *time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, namespace, description, base_url, definition, metadata,
		       created_at, updated_at, deleted_at
		FROM api_tool_definitions
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID,
	).Scan(
		&at.ID, &at.TenantID, &at.Name, &at.Namespace, &at.Description,
		&at.BaseURL, &defJSON, &metadata,
		&at.CreatedAt, &at.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying api tool definition: %w", err)
	}
	at.DeletedAt = deletedAt
	if metadata != nil {
		at.Metadata = json.RawMessage(metadata)
	}
	var dd apiToolDefData
	if err := json.Unmarshal(defJSON, &dd); err != nil {
		return nil, fmt.Errorf("unmarshaling definition: %w", err)
	}
	at.DefaultHeaders = dd.DefaultHeaders
	at.DefaultTimeout = dd.DefaultTimeout
	at.Retry = dd.Retry
	at.Endpoints = dd.Endpoints
	return at, nil
}

func (s *PostgresStore) ListAPITools(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.APIToolDefinition, string, error) {
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	var rows pgx.Rows
	var err error
	if namespace != "" {
		if cursor == "" {
			rows, err = s.pool.Query(ctx, `
				SELECT id, tenant_id, name, namespace, description, base_url, definition, metadata,
				       created_at, updated_at, deleted_at
				FROM api_tool_definitions
				WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL
				ORDER BY id LIMIT $3`, tenantID, namespace, fetchLimit)
		} else {
			rows, err = s.pool.Query(ctx, `
				SELECT id, tenant_id, name, namespace, description, base_url, definition, metadata,
				       created_at, updated_at, deleted_at
				FROM api_tool_definitions
				WHERE tenant_id = $1 AND namespace = $2 AND deleted_at IS NULL AND id > $3
				ORDER BY id LIMIT $4`, tenantID, namespace, cursor, fetchLimit)
		}
	} else {
		if cursor == "" {
			rows, err = s.pool.Query(ctx, `
				SELECT id, tenant_id, name, namespace, description, base_url, definition, metadata,
				       created_at, updated_at, deleted_at
				FROM api_tool_definitions
				WHERE tenant_id = $1 AND deleted_at IS NULL
				ORDER BY id LIMIT $2`, tenantID, fetchLimit)
		} else {
			rows, err = s.pool.Query(ctx, `
				SELECT id, tenant_id, name, namespace, description, base_url, definition, metadata,
				       created_at, updated_at, deleted_at
				FROM api_tool_definitions
				WHERE tenant_id = $1 AND deleted_at IS NULL AND id > $2
				ORDER BY id LIMIT $3`, tenantID, cursor, fetchLimit)
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing api tool definitions: %w", err)
	}
	defer rows.Close()

	var tools []*model.APIToolDefinition
	for rows.Next() {
		at := &model.APIToolDefinition{}
		var defJSON, metadata []byte
		var deletedAt *time.Time
		if err := rows.Scan(
			&at.ID, &at.TenantID, &at.Name, &at.Namespace, &at.Description,
			&at.BaseURL, &defJSON, &metadata,
			&at.CreatedAt, &at.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scanning api tool definition row: %w", err)
		}
		at.DeletedAt = deletedAt
		if metadata != nil {
			at.Metadata = json.RawMessage(metadata)
		}
		var dd apiToolDefData
		if err := json.Unmarshal(defJSON, &dd); err != nil {
			return nil, "", fmt.Errorf("unmarshaling definition: %w", err)
		}
		at.DefaultHeaders = dd.DefaultHeaders
		at.DefaultTimeout = dd.DefaultTimeout
		at.Retry = dd.Retry
		at.Endpoints = dd.Endpoints
		tools = append(tools, at)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterating api tool definition rows: %w", err)
	}

	nextCursor := ""
	if len(tools) > limit {
		nextCursor = tools[limit-1].ID
		tools = tools[:limit]
	}
	return tools, nextCursor, nil
}

func (s *PostgresStore) UpdateAPITool(ctx context.Context, at *model.APIToolDefinition) error {
	defData := apiToolDefData{
		DefaultHeaders: at.DefaultHeaders,
		DefaultTimeout: at.DefaultTimeout,
		Retry:          at.Retry,
		Endpoints:      at.Endpoints,
	}
	defJSON, err := json.Marshal(defData)
	if err != nil {
		return fmt.Errorf("marshaling definition: %w", err)
	}
	at.UpdatedAt = time.Now().UTC()

	tag, err := s.pool.Exec(ctx, `
		UPDATE api_tool_definitions
		SET name = $1, namespace = $2, description = $3, base_url = $4,
		    definition = $5, metadata = $6, updated_at = $7
		WHERE id = $8 AND tenant_id = $9 AND deleted_at IS NULL`,
		at.Name, at.Namespace, at.Description, at.BaseURL,
		defJSON, nullableJSON(at.Metadata), at.UpdatedAt,
		at.ID, at.TenantID,
	)
	if err != nil {
		return fmt.Errorf("updating api tool definition: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("api tool definition not found: %s", at.ID)
	}
	return nil
}

func (s *PostgresStore) DeleteAPITool(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_tool_definitions SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID)
	if err != nil {
		return fmt.Errorf("soft-deleting api tool definition: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("api tool definition not found: %s", id)
	}
	return nil
}

// --- Executions ---

func (s *PostgresStore) CreateExecution(ctx context.Context, exec *model.Execution) error {
	now := time.Now().UTC()
	exec.CreatedAt = now
	exec.UpdatedAt = now

	iterCountsJSON, err := json.Marshal(exec.IterationCounts)
	if err != nil {
		return fmt.Errorf("marshaling iteration_counts: %w", err)
	}
	var errorJSON []byte
	if exec.Error != nil {
		errorJSON, err = json.Marshal(exec.Error)
		if err != nil {
			return fmt.Errorf("marshaling error: %w", err)
		}
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO executions (id, tenant_id, graph_id, graph_version, status, input, output, state, error,
		                        iteration_counts, started_at, completed_at, timeout_seconds, trigger,
		                        correlation_id, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		exec.ID, exec.TenantID, exec.GraphID, exec.GraphVersion, string(exec.Status),
		nullableJSON(exec.Input), nullableJSON(exec.Output), nullableJSON(exec.State),
		nullableBytes(errorJSON), iterCountsJSON,
		exec.StartedAt, exec.CompletedAt, exec.TimeoutSeconds,
		string(exec.Trigger), exec.CorrelationID, nullableJSON(exec.Metadata),
		exec.CreatedAt, exec.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting execution: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetExecution(ctx context.Context, tenantID, id string) (*model.Execution, error) {
	exec := &model.Execution{}
	var status, trigger string
	var input, output, state, errorJSON, iterCountsJSON, metadata []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, graph_id, graph_version, status, input, output, state, error,
		       iteration_counts, started_at, completed_at, timeout_seconds, trigger,
		       correlation_id, metadata, created_at, updated_at
		FROM executions
		WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	).Scan(
		&exec.ID, &exec.TenantID, &exec.GraphID, &exec.GraphVersion, &status,
		&input, &output, &state, &errorJSON,
		&iterCountsJSON, &exec.StartedAt, &exec.CompletedAt, &exec.TimeoutSeconds,
		&trigger, &exec.CorrelationID, &metadata,
		&exec.CreatedAt, &exec.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("execution not found: %s", id)
		}
		return nil, fmt.Errorf("querying execution: %w", err)
	}
	exec.Status = model.ExecutionStatus(status)
	exec.Trigger = model.ExecutionTrigger(trigger)
	if input != nil {
		exec.Input = json.RawMessage(input)
	}
	if output != nil {
		exec.Output = json.RawMessage(output)
	}
	if state != nil {
		exec.State = json.RawMessage(state)
	}
	if metadata != nil {
		exec.Metadata = json.RawMessage(metadata)
	}
	if errorJSON != nil {
		exec.Error = &model.ExecutionError{}
		if err := json.Unmarshal(errorJSON, exec.Error); err != nil {
			return nil, fmt.Errorf("unmarshaling error: %w", err)
		}
	}
	if iterCountsJSON != nil {
		if err := json.Unmarshal(iterCountsJSON, &exec.IterationCounts); err != nil {
			return nil, fmt.Errorf("unmarshaling iteration_counts: %w", err)
		}
	}
	return exec, nil
}

func (s *PostgresStore) ListExecutions(ctx context.Context, tenantID string, graphID string, status string, cursor string, limit int) ([]*model.Execution, string, error) {
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	// Build dynamic query based on filters.
	query := `SELECT id, tenant_id, graph_id, graph_version, status, input, output, state, error,
	                 iteration_counts, started_at, completed_at, timeout_seconds, trigger,
	                 correlation_id, metadata, created_at, updated_at
	          FROM executions WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if graphID != "" {
		query += fmt.Sprintf(" AND graph_id = $%d", argIdx)
		args = append(args, graphID)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if cursor != "" {
		query += fmt.Sprintf(" AND id > $%d", argIdx)
		args = append(args, cursor)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY id LIMIT $%d", argIdx)
	args = append(args, fetchLimit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("listing executions: %w", err)
	}
	defer rows.Close()

	var executions []*model.Execution
	for rows.Next() {
		exec := &model.Execution{}
		var statusStr, trigger string
		var input, output, stateJSON, errorJSON, iterCountsJSON, metadata []byte

		if err := rows.Scan(
			&exec.ID, &exec.TenantID, &exec.GraphID, &exec.GraphVersion, &statusStr,
			&input, &output, &stateJSON, &errorJSON,
			&iterCountsJSON, &exec.StartedAt, &exec.CompletedAt, &exec.TimeoutSeconds,
			&trigger, &exec.CorrelationID, &metadata,
			&exec.CreatedAt, &exec.UpdatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scanning execution row: %w", err)
		}
		exec.Status = model.ExecutionStatus(statusStr)
		exec.Trigger = model.ExecutionTrigger(trigger)
		if input != nil {
			exec.Input = json.RawMessage(input)
		}
		if output != nil {
			exec.Output = json.RawMessage(output)
		}
		if stateJSON != nil {
			exec.State = json.RawMessage(stateJSON)
		}
		if metadata != nil {
			exec.Metadata = json.RawMessage(metadata)
		}
		if errorJSON != nil {
			exec.Error = &model.ExecutionError{}
			if err := json.Unmarshal(errorJSON, exec.Error); err != nil {
				return nil, "", fmt.Errorf("unmarshaling error: %w", err)
			}
		}
		if iterCountsJSON != nil {
			if err := json.Unmarshal(iterCountsJSON, &exec.IterationCounts); err != nil {
				return nil, "", fmt.Errorf("unmarshaling iteration_counts: %w", err)
			}
		}
		executions = append(executions, exec)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterating execution rows: %w", err)
	}

	nextCursor := ""
	if len(executions) > limit {
		nextCursor = executions[limit-1].ID
		executions = executions[:limit]
	}
	return executions, nextCursor, nil
}

func (s *PostgresStore) UpdateExecution(ctx context.Context, exec *model.Execution) error {
	exec.UpdatedAt = time.Now().UTC()

	iterCountsJSON, err := json.Marshal(exec.IterationCounts)
	if err != nil {
		return fmt.Errorf("marshaling iteration_counts: %w", err)
	}
	var errorJSON []byte
	if exec.Error != nil {
		errorJSON, err = json.Marshal(exec.Error)
		if err != nil {
			return fmt.Errorf("marshaling error: %w", err)
		}
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE executions
		SET status = $1, output = $2, state = $3, error = $4, iteration_counts = $5,
		    started_at = $6, completed_at = $7, metadata = $8, updated_at = $9
		WHERE id = $10 AND tenant_id = $11`,
		string(exec.Status), nullableJSON(exec.Output), nullableJSON(exec.State),
		nullableBytes(errorJSON), iterCountsJSON,
		exec.StartedAt, exec.CompletedAt, nullableJSON(exec.Metadata), exec.UpdatedAt,
		exec.ID, exec.TenantID,
	)
	if err != nil {
		return fmt.Errorf("updating execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("execution not found: %s", exec.ID)
	}
	return nil
}

// --- Execution Steps ---

func (s *PostgresStore) InsertExecutionStep(ctx context.Context, step *model.ExecutionStep) error {
	step.CreatedAt = time.Now().UTC()

	var llmUsageJSON, llmDebugJSON []byte
	var err error
	if step.LLMUsage != nil {
		llmUsageJSON, err = json.Marshal(step.LLMUsage)
		if err != nil {
			return fmt.Errorf("marshaling llm_usage: %w", err)
		}
	}
	if step.LLMDebug != nil {
		llmDebugJSON, err = json.Marshal(step.LLMDebug)
		if err != nil {
			return fmt.Errorf("marshaling llm_debug: %w", err)
		}
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO execution_steps (id, execution_id, node_id, node_type, iteration, status,
		                             input, output, state_before, state_after, error, attempt,
		                             started_at, completed_at, duration_ms, llm_usage, llm_debug, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		step.ID, step.ExecutionID, step.NodeID, step.NodeType, step.Iteration, string(step.Status),
		nullableJSON(step.Input), nullableJSON(step.Output),
		nullableJSON(step.StateBefore), nullableJSON(step.StateAfter),
		nullableJSON(step.Error), step.Attempt,
		step.StartedAt, step.CompletedAt, step.DurationMs,
		nullableBytes(llmUsageJSON), nullableBytes(llmDebugJSON), step.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting execution step: %w", err)
	}
	return nil
}

func (s *PostgresStore) ListExecutionSteps(ctx context.Context, executionID string) ([]*model.ExecutionStep, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, execution_id, node_id, node_type, iteration, status,
		       input, output, state_before, state_after, error, attempt,
		       started_at, completed_at, duration_ms, llm_usage, llm_debug, created_at
		FROM execution_steps
		WHERE execution_id = $1
		ORDER BY created_at`, executionID)
	if err != nil {
		return nil, fmt.Errorf("listing execution steps: %w", err)
	}
	defer rows.Close()

	var steps []*model.ExecutionStep
	for rows.Next() {
		step := &model.ExecutionStep{}
		var statusStr string
		var input, output, stateBefore, stateAfter, errorJSON, llmUsageJSON, llmDebugJSON []byte
		if err := rows.Scan(
			&step.ID, &step.ExecutionID, &step.NodeID, &step.NodeType, &step.Iteration, &statusStr,
			&input, &output, &stateBefore, &stateAfter, &errorJSON, &step.Attempt,
			&step.StartedAt, &step.CompletedAt, &step.DurationMs, &llmUsageJSON, &llmDebugJSON, &step.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning execution step row: %w", err)
		}
		step.Status = model.StepStatus(statusStr)
		if input != nil {
			step.Input = json.RawMessage(input)
		}
		if output != nil {
			step.Output = json.RawMessage(output)
		}
		if stateBefore != nil {
			step.StateBefore = json.RawMessage(stateBefore)
		}
		if stateAfter != nil {
			step.StateAfter = json.RawMessage(stateAfter)
		}
		if errorJSON != nil {
			step.Error = json.RawMessage(errorJSON)
		}
		if llmUsageJSON != nil {
			step.LLMUsage = &model.LLMUsage{}
			if err := json.Unmarshal(llmUsageJSON, step.LLMUsage); err != nil {
				return nil, fmt.Errorf("unmarshaling llm_usage: %w", err)
			}
		}
		if llmDebugJSON != nil {
			step.LLMDebug = &model.LLMDebugTrace{}
			if err := json.Unmarshal(llmDebugJSON, step.LLMDebug); err != nil {
				return nil, fmt.Errorf("unmarshaling llm_debug: %w", err)
			}
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating execution step rows: %w", err)
	}
	return steps, nil
}

// --- Helpers ---

// nullableJSON returns nil if the json.RawMessage is nil or empty, otherwise returns the bytes.
func nullableJSON(data json.RawMessage) []byte {
	if len(data) == 0 {
		return nil
	}
	return []byte(data)
}

// nullableBytes returns nil if the slice is empty.
func nullableBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	return data
}
