package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// MockStore is an in-memory, thread-safe implementation of model.Store.
type MockStore struct {
	mu sync.RWMutex

	graphs          map[string]*model.Graph
	schemas         map[string]*model.SchemaLibrary
	promptTemplates map[string]*model.PromptLibrary
	providerConfigs map[string]*model.ProviderConfigLibrary
	apiTools        map[string]*model.APIToolDefinition
	executions      map[string]*model.Execution
	executionSteps  map[string][]*model.ExecutionStep // keyed by execution ID
}

var _ model.Store = (*MockStore)(nil)

// NewMockStore creates a MockStore with initialized maps.
func NewMockStore() *MockStore {
	return &MockStore{
		graphs:          make(map[string]*model.Graph),
		schemas:         make(map[string]*model.SchemaLibrary),
		promptTemplates: make(map[string]*model.PromptLibrary),
		providerConfigs: make(map[string]*model.ProviderConfigLibrary),
		apiTools:        make(map[string]*model.APIToolDefinition),
		executions:      make(map[string]*model.Execution),
		executionSteps:  make(map[string][]*model.ExecutionStep),
	}
}

// compositeKey builds a store key from tenant + id.
func compositeKey(tenantID, id string) string {
	return tenantID + "/" + id
}

// --- Graphs ---

func (s *MockStore) CreateGraph(ctx context.Context, graph *model.Graph) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(graph.TenantID, graph.ID)
	if _, exists := s.graphs[key]; exists {
		return fmt.Errorf("graph %q already exists", graph.ID)
	}
	s.graphs[key] = graph
	return nil
}

func (s *MockStore) GetGraph(ctx context.Context, tenantID, id string) (*model.Graph, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.graphs[compositeKey(tenantID, id)]
	if !ok {
		return nil, fmt.Errorf("graph %q not found", id)
	}
	return g, nil
}

func (s *MockStore) ListGraphs(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.Graph, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Graph
	for _, g := range s.graphs {
		if g.TenantID != tenantID {
			continue
		}
		if namespace != "" && g.Namespace != namespace {
			continue
		}
		result = append(result, g)
	}
	return applyPagination(result, cursor, limit)
}

func (s *MockStore) UpdateGraph(ctx context.Context, graph *model.Graph) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(graph.TenantID, graph.ID)
	if _, exists := s.graphs[key]; !exists {
		return fmt.Errorf("graph %q not found", graph.ID)
	}
	s.graphs[key] = graph
	return nil
}

func (s *MockStore) DeleteGraph(ctx context.Context, tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(tenantID, id)
	if _, exists := s.graphs[key]; !exists {
		return fmt.Errorf("graph %q not found", id)
	}
	delete(s.graphs, key)
	return nil
}

// --- Schemas ---

func (s *MockStore) CreateSchema(ctx context.Context, schema *model.SchemaLibrary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(schema.TenantID, schema.ID)
	if _, exists := s.schemas[key]; exists {
		return fmt.Errorf("schema %q already exists", schema.ID)
	}
	s.schemas[key] = schema
	return nil
}

func (s *MockStore) GetSchema(ctx context.Context, tenantID, id string) (*model.SchemaLibrary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.schemas[compositeKey(tenantID, id)]
	if !ok {
		return nil, fmt.Errorf("schema %q not found", id)
	}
	return v, nil
}

func (s *MockStore) ListSchemas(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.SchemaLibrary, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.SchemaLibrary
	for _, v := range s.schemas {
		if v.TenantID != tenantID {
			continue
		}
		if namespace != "" && v.Namespace != namespace {
			continue
		}
		result = append(result, v)
	}
	return applyPagination(result, cursor, limit)
}

func (s *MockStore) UpdateSchema(ctx context.Context, schema *model.SchemaLibrary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(schema.TenantID, schema.ID)
	if _, exists := s.schemas[key]; !exists {
		return fmt.Errorf("schema %q not found", schema.ID)
	}
	s.schemas[key] = schema
	return nil
}

func (s *MockStore) DeleteSchema(ctx context.Context, tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(tenantID, id)
	if _, exists := s.schemas[key]; !exists {
		return fmt.Errorf("schema %q not found", id)
	}
	delete(s.schemas, key)
	return nil
}

// --- Prompt Templates ---

func (s *MockStore) CreatePromptTemplate(ctx context.Context, pt *model.PromptLibrary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(pt.TenantID, pt.ID)
	if _, exists := s.promptTemplates[key]; exists {
		return fmt.Errorf("prompt template %q already exists", pt.ID)
	}
	s.promptTemplates[key] = pt
	return nil
}

func (s *MockStore) GetPromptTemplate(ctx context.Context, tenantID, id string) (*model.PromptLibrary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.promptTemplates[compositeKey(tenantID, id)]
	if !ok {
		return nil, fmt.Errorf("prompt template %q not found", id)
	}
	return v, nil
}

func (s *MockStore) ListPromptTemplates(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.PromptLibrary, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.PromptLibrary
	for _, v := range s.promptTemplates {
		if v.TenantID != tenantID {
			continue
		}
		if namespace != "" && v.Namespace != namespace {
			continue
		}
		result = append(result, v)
	}
	return applyPagination(result, cursor, limit)
}

func (s *MockStore) UpdatePromptTemplate(ctx context.Context, pt *model.PromptLibrary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(pt.TenantID, pt.ID)
	if _, exists := s.promptTemplates[key]; !exists {
		return fmt.Errorf("prompt template %q not found", pt.ID)
	}
	s.promptTemplates[key] = pt
	return nil
}

func (s *MockStore) DeletePromptTemplate(ctx context.Context, tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(tenantID, id)
	if _, exists := s.promptTemplates[key]; !exists {
		return fmt.Errorf("prompt template %q not found", id)
	}
	delete(s.promptTemplates, key)
	return nil
}

// --- Provider Configs ---

func (s *MockStore) CreateProviderConfig(ctx context.Context, pc *model.ProviderConfigLibrary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(pc.TenantID, pc.ID)
	if _, exists := s.providerConfigs[key]; exists {
		return fmt.Errorf("provider config %q already exists", pc.ID)
	}
	s.providerConfigs[key] = pc
	return nil
}

func (s *MockStore) GetProviderConfig(ctx context.Context, tenantID, id string) (*model.ProviderConfigLibrary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.providerConfigs[compositeKey(tenantID, id)]
	if !ok {
		return nil, fmt.Errorf("provider config %q not found", id)
	}
	return v, nil
}

func (s *MockStore) ListProviderConfigs(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.ProviderConfigLibrary, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.ProviderConfigLibrary
	for _, v := range s.providerConfigs {
		if v.TenantID != tenantID {
			continue
		}
		if namespace != "" && v.Namespace != namespace {
			continue
		}
		result = append(result, v)
	}
	return applyPagination(result, cursor, limit)
}

func (s *MockStore) UpdateProviderConfig(ctx context.Context, pc *model.ProviderConfigLibrary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(pc.TenantID, pc.ID)
	if _, exists := s.providerConfigs[key]; !exists {
		return fmt.Errorf("provider config %q not found", pc.ID)
	}
	s.providerConfigs[key] = pc
	return nil
}

func (s *MockStore) DeleteProviderConfig(ctx context.Context, tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(tenantID, id)
	if _, exists := s.providerConfigs[key]; !exists {
		return fmt.Errorf("provider config %q not found", id)
	}
	delete(s.providerConfigs, key)
	return nil
}

// --- API Tool Definitions ---

func (s *MockStore) CreateAPITool(ctx context.Context, at *model.APIToolDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(at.TenantID, at.ID)
	if _, exists := s.apiTools[key]; exists {
		return fmt.Errorf("api tool %q already exists", at.ID)
	}
	s.apiTools[key] = at
	return nil
}

func (s *MockStore) GetAPITool(ctx context.Context, tenantID, id string) (*model.APIToolDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.apiTools[compositeKey(tenantID, id)]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (s *MockStore) ListAPITools(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*model.APIToolDefinition, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.APIToolDefinition
	for _, v := range s.apiTools {
		if v.TenantID != tenantID {
			continue
		}
		if namespace != "" && v.Namespace != namespace {
			continue
		}
		result = append(result, v)
	}
	return applyPagination(result, cursor, limit)
}

func (s *MockStore) UpdateAPITool(ctx context.Context, at *model.APIToolDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(at.TenantID, at.ID)
	if _, exists := s.apiTools[key]; !exists {
		return fmt.Errorf("api tool %q not found", at.ID)
	}
	s.apiTools[key] = at
	return nil
}

func (s *MockStore) DeleteAPITool(ctx context.Context, tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(tenantID, id)
	if _, exists := s.apiTools[key]; !exists {
		return fmt.Errorf("api tool %q not found", id)
	}
	delete(s.apiTools, key)
	return nil
}

// --- Executions ---

func (s *MockStore) CreateExecution(ctx context.Context, exec *model.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(exec.TenantID, exec.ID)
	if _, exists := s.executions[key]; exists {
		return fmt.Errorf("execution %q already exists", exec.ID)
	}
	s.executions[key] = exec
	return nil
}

func (s *MockStore) GetExecution(ctx context.Context, tenantID, id string) (*model.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.executions[compositeKey(tenantID, id)]
	if !ok {
		return nil, fmt.Errorf("execution %q not found", id)
	}
	return v, nil
}

func (s *MockStore) ListExecutions(ctx context.Context, tenantID string, graphID string, status string, cursor string, limit int) ([]*model.Execution, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Execution
	for _, v := range s.executions {
		if v.TenantID != tenantID {
			continue
		}
		if graphID != "" && v.GraphID != graphID {
			continue
		}
		if status != "" && string(v.Status) != status {
			continue
		}
		result = append(result, v)
	}
	return applyPagination(result, cursor, limit)
}

func (s *MockStore) UpdateExecution(ctx context.Context, exec *model.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(exec.TenantID, exec.ID)
	if _, exists := s.executions[key]; !exists {
		return fmt.Errorf("execution %q not found", exec.ID)
	}
	s.executions[key] = exec
	return nil
}

// --- Execution Steps ---

func (s *MockStore) InsertExecutionStep(ctx context.Context, step *model.ExecutionStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executionSteps[step.ExecutionID] = append(s.executionSteps[step.ExecutionID], step)
	return nil
}

func (s *MockStore) ListExecutionSteps(ctx context.Context, executionID string) ([]*model.ExecutionStep, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	steps := s.executionSteps[executionID]
	// Return a copy to avoid data races on the slice.
	result := make([]*model.ExecutionStep, len(steps))
	copy(result, steps)
	return result, nil
}

// applyPagination is a generic helper for simple cursor-based pagination on in-memory slices.
// For the mock, cursor is treated as an offset index string. An empty cursor starts at 0.
func applyPagination[T any](items []*T, cursor string, limit int) ([]*T, string, error) {
	if limit <= 0 {
		limit = 100
	}

	start := 0
	if cursor != "" {
		n := 0
		for _, c := range cursor {
			n = n*10 + int(c-'0')
		}
		start = n
	}

	if start >= len(items) {
		return nil, "", nil
	}

	end := start + limit
	if end > len(items) {
		end = len(items)
	}

	var nextCursor string
	if end < len(items) {
		nextCursor = fmt.Sprintf("%d", end)
	}

	return items[start:end], nextCursor, nil
}
