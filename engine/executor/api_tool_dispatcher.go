package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

// pathTemplateRegex matches {{input.x}} patterns in URL paths.
var pathTemplateRegex = regexp.MustCompile(`\{\{\s*input\.(\w+)\s*\}\}`)

const (
	defaultAPITimeout     = 30 * time.Second
	defaultMaxResponseLen = 1 << 20 // 1MB
)

// APIToolDispatcher resolves API tool definitions and executes HTTP endpoint calls.
type APIToolDispatcher struct {
	store      model.Store
	httpClient *http.Client
	mu         sync.RWMutex
	cache      map[string]*model.APIToolDefinition // tenantID/id -> definition
	logger     *slog.Logger
}

// NewAPIToolDispatcher creates a new dispatcher. The cache is scoped to one execution.
func NewAPIToolDispatcher(store model.Store, logger *slog.Logger) *APIToolDispatcher {
	return &APIToolDispatcher{
		store:      store,
		httpClient: &http.Client{},
		cache:      make(map[string]*model.APIToolDefinition),
		logger:     logger,
	}
}

// ResolveDefinition looks up an API tool definition by ID (cached per execution).
func (d *APIToolDispatcher) ResolveDefinition(ctx context.Context, tenantID, apiToolID string) (*model.APIToolDefinition, error) {
	cacheKey := tenantID + "/" + apiToolID

	d.mu.RLock()
	if def, ok := d.cache[cacheKey]; ok {
		d.mu.RUnlock()
		return def, nil
	}
	d.mu.RUnlock()

	def, err := d.store.GetAPITool(ctx, tenantID, apiToolID)
	if err != nil {
		return nil, fmt.Errorf("resolving API tool %q: %w", apiToolID, err)
	}
	if def == nil {
		return nil, fmt.Errorf("API tool %q not found", apiToolID)
	}

	d.mu.Lock()
	d.cache[cacheKey] = def
	d.mu.Unlock()

	return def, nil
}

// CallEndpoint executes a single API endpoint call.
func (d *APIToolDispatcher) CallEndpoint(
	ctx context.Context,
	tenantID string,
	route model.ToolRoute,
	toolName string,
	args map[string]any,
) (*model.ToolResult, error) {
	def, err := d.ResolveDefinition(ctx, tenantID, route.APIToolID)
	if err != nil {
		return nil, err
	}

	ep := FindEndpoint(def, route.APIEndpoint)
	if ep == nil {
		return nil, fmt.Errorf("endpoint %q not found in API tool %q", route.APIEndpoint, route.APIToolID)
	}

	return d.executeHTTPCall(ctx, def, ep, route, args)
}

// FindEndpoint returns the named endpoint from a definition, or nil if not found.
func FindEndpoint(def *model.APIToolDefinition, name string) *model.APIEndpoint {
	for i := range def.Endpoints {
		if def.Endpoints[i].Name == name {
			return &def.Endpoints[i]
		}
	}
	return nil
}

func (d *APIToolDispatcher) executeHTTPCall(
	ctx context.Context,
	def *model.APIToolDefinition,
	ep *model.APIEndpoint,
	route model.ToolRoute,
	args map[string]any,
) (*model.ToolResult, error) {
	// Resolve path templates and separate consumed args from remaining.
	resolvedPath, remaining := resolvePathTemplate(ep.Path, args)

	// Build full URL.
	fullURL := strings.TrimRight(def.BaseURL, "/") + resolvedPath

	// Determine request mapping mode.
	mappingMode := "json_body"
	if ep.RequestMapping != nil && ep.RequestMapping.Mode != "" {
		mappingMode = ep.RequestMapping.Mode
	}

	// Build HTTP request.
	var req *http.Request
	var err error

	switch mappingMode {
	case "json_body":
		req, err = buildJSONRequest(ctx, ep.Method, fullURL, remaining)
	case "form":
		req, err = buildFormRequest(ctx, ep.Method, fullURL, remaining)
	case "query_params":
		req, err = buildQueryRequest(ctx, ep.Method, fullURL, remaining)
	case "path_and_body":
		// Path params already consumed; remaining go in body.
		req, err = buildJSONRequest(ctx, ep.Method, fullURL, remaining)
	default:
		return nil, fmt.Errorf("unknown request_mapping mode: %s", mappingMode)
	}
	if err != nil {
		return nil, fmt.Errorf("building HTTP request: %w", err)
	}

	// Merge headers: definition defaults < endpoint headers < route headers.
	mergeHeaders(req, def.DefaultHeaders)
	mergeHeaders(req, ep.Headers)
	mergeHeaders(req, route.Headers)

	// Determine timeout.
	timeout := defaultAPITimeout
	if def.DefaultTimeout > 0 {
		timeout = time.Duration(def.DefaultTimeout) * time.Millisecond
	}
	if ep.TimeoutMs != nil && *ep.TimeoutMs > 0 {
		timeout = time.Duration(*ep.TimeoutMs) * time.Millisecond
	}
	if route.TimeoutSeconds != nil && *route.TimeoutSeconds > 0 {
		timeout = time.Duration(*route.TimeoutSeconds) * time.Second
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req = req.WithContext(callCtx)

	// Execute with optional retry.
	resp, err := d.executeWithRetry(req, def.Retry)
	if err != nil {
		return &model.ToolResult{
			IsError: true,
			Error:   fmt.Sprintf("HTTP request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body (capped at 1MB).
	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultMaxResponseLen))
	if err != nil {
		return &model.ToolResult{
			IsError: true,
			Error:   fmt.Sprintf("reading response body: %v", err),
		}, nil
	}

	// Check HTTP error.
	if resp.StatusCode >= 400 {
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return &model.ToolResult{
			IsError: true,
			Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, preview),
		}, nil
	}

	// Map response.
	responseMode := "json_body"
	if ep.ResponseMapping != nil && ep.ResponseMapping.Mode != "" {
		responseMode = ep.ResponseMapping.Mode
	}

	var content any
	switch responseMode {
	case "json_body":
		content, err = mapResponseJSON(body)
		if err != nil {
			content = string(body) // fallback to raw string
		}
	case "text":
		content = string(body)
	case "jq":
		expr := ""
		if ep.ResponseMapping != nil {
			expr = ep.ResponseMapping.Expression
		}
		content, err = mapResponseJQ(body, expr)
		if err != nil {
			content = string(body) // fallback
		}
	case "headers_and_body":
		content = mapResponseHeadersAndBody(resp, body)
	default:
		content = string(body)
	}

	return &model.ToolResult{
		Content: content,
		IsError: false,
	}, nil
}

// resolvePathTemplate replaces {{input.x}} in a path with values from args.
// Returns the resolved path and the remaining args (those not consumed by path).
func resolvePathTemplate(path string, args map[string]any) (string, map[string]any) {
	remaining := make(map[string]any)
	for k, v := range args {
		remaining[k] = v
	}

	resolved := pathTemplateRegex.ReplaceAllStringFunc(path, func(match string) string {
		sub := pathTemplateRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		key := sub[1]
		if val, ok := remaining[key]; ok {
			delete(remaining, key)
			return url.PathEscape(fmt.Sprintf("%v", val))
		}
		return match
	})

	return resolved, remaining
}

func buildJSONRequest(ctx context.Context, method, fullURL string, args map[string]any) (*http.Request, error) {
	var bodyReader io.Reader
	if len(args) > 0 && method != "GET" && method != "DELETE" {
		data, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func buildFormRequest(ctx context.Context, method, fullURL string, args map[string]any) (*http.Request, error) {
	form := url.Values{}
	for k, v := range args {
		form.Set(k, fmt.Sprintf("%v", v))
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func buildQueryRequest(ctx context.Context, method, fullURL string, args map[string]any) (*http.Request, error) {
	u, err := url.Parse(fullURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range args {
		q.Set(k, fmt.Sprintf("%v", v))
	}
	u.RawQuery = q.Encode()
	return http.NewRequestWithContext(ctx, method, u.String(), nil)
}

func mergeHeaders(req *http.Request, headers []model.HeaderConfig) {
	for _, hc := range headers {
		val := hc.Value
		if val != "" {
			req.Header.Set(hc.Name, val)
		}
	}
}

func (d *APIToolDispatcher) executeWithRetry(req *http.Request, retryCfg *model.RetryConfig) (*http.Response, error) {
	maxAttempts := 1
	if retryCfg != nil && retryCfg.MaxRetries > 0 {
		maxAttempts = retryCfg.MaxRetries + 1
	}

	retryStatuses := make(map[int]bool)
	if retryCfg != nil {
		for _, s := range retryCfg.RetryOnStatus {
			retryStatuses[s] = true
		}
	}

	backoffMs := 1000
	if retryCfg != nil && retryCfg.BackoffMs > 0 {
		backoffMs = retryCfg.BackoffMs
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter.
			delay := time.Duration(float64(backoffMs)*math.Pow(2, float64(attempt-1))) * time.Millisecond
			jitter := time.Duration(rand.Int63n(int64(delay / 4)))
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay + jitter):
			}

			d.logger.Info("retrying API tool call",
				"attempt", attempt+1,
				"url", req.URL.String(),
			)
		}

		// Clone the request body for retries.
		var retryReq *http.Request
		if attempt == 0 {
			retryReq = req
		} else {
			var err error
			retryReq, err = http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), req.Body)
			if err != nil {
				return nil, err
			}
			retryReq.Header = req.Header
		}

		resp, err := d.httpClient.Do(retryReq)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts-1 {
				continue
			}
			return nil, err
		}

		// Check if we should retry.
		if len(retryStatuses) > 0 && retryStatuses[resp.StatusCode] && attempt < maxAttempts-1 {
			resp.Body.Close()
			lastResp = resp
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}

func mapResponseJSON(body []byte) (any, error) {
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func mapResponseJQ(body []byte, expr string) (any, error) {
	if expr == "" {
		return mapResponseJSON(body)
	}

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	// Simple dot-path extraction (e.g., ".data", ".data.id", ".items[0]").
	parts := strings.Split(strings.TrimPrefix(expr, "."), ".")
	current := parsed
	for _, part := range parts {
		if part == "" {
			continue
		}
		switch m := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = m[part]
			if !ok {
				return nil, fmt.Errorf("jq: key %q not found", part)
			}
		default:
			return nil, fmt.Errorf("jq: cannot traverse %q on type %T", part, current)
		}
	}

	return current, nil
}

func mapResponseHeadersAndBody(resp *http.Response, body []byte) map[string]any {
	headers := make(map[string]string)
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}

	var bodyParsed any
	if err := json.Unmarshal(body, &bodyParsed); err != nil {
		bodyParsed = string(body)
	}

	return map[string]any{
		"headers": headers,
		"body":    bodyParsed,
	}
}
