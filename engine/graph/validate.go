// Package graph implements graph validation for Brockley.
// It validates graph structure, port compatibility, typing rules,
// and execution order planning.
package graph

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/brockleyai/brockleyai/internal/model"
)

// ValidationResult contains the result of graph validation.
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
}

// ValidationError describes a single validation issue.
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	NodeID  string `json:"node_id,omitempty"`
	EdgeID  string `json:"edge_id,omitempty"`
	Field   string `json:"field,omitempty"`
}

// Validate checks a graph for structural and type errors.
func Validate(g *model.Graph) *ValidationResult {
	v := &validator{
		graph:   g,
		nodeMap: make(map[string]*model.Node),
		result:  &ValidationResult{Valid: true},
	}
	v.validate()
	return v.result
}

type validator struct {
	graph   *model.Graph
	nodeMap map[string]*model.Node
	result  *ValidationResult
}

func (v *validator) addError(code, message string, nodeID, edgeID, field string) {
	v.result.Valid = false
	v.result.Errors = append(v.result.Errors, ValidationError{
		Code:    code,
		Message: message,
		NodeID:  nodeID,
		EdgeID:  edgeID,
		Field:   field,
	})
}

func (v *validator) addWarning(code, message string, nodeID string) {
	v.result.Warnings = append(v.result.Warnings, ValidationError{
		Code:    code,
		Message: message,
		NodeID:  nodeID,
	})
}

func (v *validator) validate() {
	// Build node map
	for i := range v.graph.Nodes {
		node := &v.graph.Nodes[i]
		if _, exists := v.nodeMap[node.ID]; exists {
			v.addError("DUPLICATE_NODE_ID", fmt.Sprintf("duplicate node ID: %s", node.ID), node.ID, "", "id")
			continue
		}
		v.nodeMap[node.ID] = node
	}

	// Must have at least one node
	if len(v.graph.Nodes) == 0 {
		v.addError("EMPTY_GRAPH", "graph must have at least one node", "", "", "nodes")
		return
	}

	// Must have at least one input node
	v.validateHasInputNode()

	// Validate each node
	for i := range v.graph.Nodes {
		v.validateNode(&v.graph.Nodes[i])
	}

	// Validate tool loop config on LLM nodes
	v.validateToolLoopConfigs()

	// Validate superagent configs
	v.validateSuperagentConfigs()

	// Validate api_tool node configs
	v.validateAPIToolNodeConfigs()

	// Validate edges
	for i := range v.graph.Edges {
		v.validateEdge(&v.graph.Edges[i])
	}

	// Validate state schema
	if v.graph.State != nil {
		v.validateState(v.graph.State)
	}

	// Validate state bindings reference valid state fields
	v.validateStateBindings()

	// Validate back-edges
	v.validateBackEdges()

	// Validate no unguarded cycles
	v.validateNoCycles()

	// Validate exclusive fan-in
	v.validateExclusiveFanIn()

	// Validate reachability
	v.validateReachability()

	// Validate required ports are wired
	v.validateRequiredPorts()
}

func (v *validator) validateHasInputNode() {
	for i := range v.graph.Nodes {
		if v.graph.Nodes[i].Type == model.NodeTypeInput {
			return
		}
	}
	v.addError("NO_INPUT_NODE", "graph must have at least one input node", "", "", "nodes")
}

func (v *validator) validateNode(node *model.Node) {
	if node.ID == "" {
		v.addError("EMPTY_NODE_ID", "node ID must not be empty", "", "", "id")
	}
	if node.Name == "" {
		v.addError("EMPTY_NODE_NAME", "node name must not be empty", node.ID, "", "name")
	}

	// Validate port schemas (strong typing)
	for j, port := range node.InputPorts {
		v.validatePortSchema(node.ID, fmt.Sprintf("input_ports[%d]", j), port)
	}
	for j, port := range node.OutputPorts {
		v.validatePortSchema(node.ID, fmt.Sprintf("output_ports[%d]", j), port)
	}

	// Validate unique port names within a node
	v.validateUniquePortNames(node)
}

func (v *validator) validatePortSchema(nodeID, field string, port model.Port) {
	if port.Name == "" {
		v.addError("EMPTY_PORT_NAME", "port name must not be empty", nodeID, "", field+".name")
		return
	}
	if len(port.Schema) == 0 {
		v.addError("MISSING_PORT_SCHEMA", fmt.Sprintf("port %q must have a schema", port.Name), nodeID, "", field+".schema")
		return
	}
	v.validateStrongTyping(nodeID, field+".schema", port.Schema)
}

func (v *validator) validateStrongTyping(nodeID, field string, schema json.RawMessage) {
	var s map[string]any
	if err := json.Unmarshal(schema, &s); err != nil {
		v.addError("INVALID_SCHEMA", fmt.Sprintf("invalid JSON Schema: %v", err), nodeID, "", field)
		return
	}

	typ, _ := s["type"].(string)

	switch typ {
	case "object":
		if _, hasProps := s["properties"]; !hasProps {
			// Check for oneOf/anyOf which are also valid
			if _, hasOneOf := s["oneOf"]; !hasOneOf {
				if _, hasAnyOf := s["anyOf"]; !hasAnyOf {
					v.addError("SCHEMA_VIOLATION", `object schema must have 'properties' (bare {"type":"object"} not allowed)`, nodeID, "", field)
				}
			}
		} else {
			// Recursively validate property schemas
			if props, ok := s["properties"].(map[string]any); ok {
				for propName, propSchema := range props {
					propBytes, _ := json.Marshal(propSchema)
					v.validateStrongTyping(nodeID, field+".properties."+propName, propBytes)
				}
			}
		}
	case "array":
		if _, hasItems := s["items"]; !hasItems {
			v.addError("SCHEMA_VIOLATION", `array schema must have 'items' (bare {"type":"array"} not allowed)`, nodeID, "", field)
		} else {
			itemBytes, _ := json.Marshal(s["items"])
			v.validateStrongTyping(nodeID, field+".items", itemBytes)
		}
	case "string", "integer", "number", "boolean":
		// Scalar types are self-describing, no further validation needed
	case "":
		// Check for oneOf/anyOf at top level
		if _, hasOneOf := s["oneOf"]; hasOneOf {
			return
		}
		if _, hasAnyOf := s["anyOf"]; hasAnyOf {
			return
		}
		if _, hasEnum := s["enum"]; hasEnum {
			return
		}
		v.addError("MISSING_TYPE", "schema must have a 'type' field", nodeID, "", field)
	}
}

func (v *validator) validateUniquePortNames(node *model.Node) {
	seen := make(map[string]bool)
	for _, p := range node.InputPorts {
		if seen[p.Name] {
			v.addError("DUPLICATE_PORT_NAME", fmt.Sprintf("duplicate input port name: %s", p.Name), node.ID, "", "input_ports")
		}
		seen[p.Name] = true
	}
	seen = make(map[string]bool)
	for _, p := range node.OutputPorts {
		if seen[p.Name] {
			v.addError("DUPLICATE_PORT_NAME", fmt.Sprintf("duplicate output port name: %s", p.Name), node.ID, "", "output_ports")
		}
		seen[p.Name] = true
	}
}

func (v *validator) validateEdge(edge *model.Edge) {
	if edge.ID == "" {
		v.addError("EMPTY_EDGE_ID", "edge ID must not be empty", "", "", "id")
		return
	}

	srcNode, srcExists := v.nodeMap[edge.SourceNodeID]
	if !srcExists {
		v.addError("INVALID_SOURCE_NODE", fmt.Sprintf("source node %q does not exist", edge.SourceNodeID), "", edge.ID, "source_node_id")
		return
	}

	_, tgtExists := v.nodeMap[edge.TargetNodeID]
	if !tgtExists {
		v.addError("INVALID_TARGET_NODE", fmt.Sprintf("target node %q does not exist", edge.TargetNodeID), "", edge.ID, "target_node_id")
		return
	}

	if edge.SourceNodeID == edge.TargetNodeID && !edge.BackEdge {
		v.addError("SELF_REFERENCE", "edge cannot reference the same node as source and target (use back_edge for loops)", "", edge.ID, "")
	}

	// Check source port exists
	if !hasPort(srcNode.OutputPorts, edge.SourcePort) {
		v.addError("INVALID_SOURCE_PORT", fmt.Sprintf("source port %q does not exist on node %q", edge.SourcePort, edge.SourceNodeID), edge.SourceNodeID, edge.ID, "source_port")
	}

	// Check target port exists
	tgtNode := v.nodeMap[edge.TargetNodeID]
	if tgtNode != nil && !hasPort(tgtNode.InputPorts, edge.TargetPort) {
		v.addError("INVALID_TARGET_PORT", fmt.Sprintf("target port %q does not exist on node %q", edge.TargetPort, edge.TargetNodeID), edge.TargetNodeID, edge.ID, "target_port")
	}
}

func (v *validator) validateState(state *model.GraphState) {
	seen := make(map[string]bool)
	for i, field := range state.Fields {
		if field.Name == "" {
			v.addError("EMPTY_STATE_FIELD", "state field name must not be empty", "", "", fmt.Sprintf("state.fields[%d].name", i))
			continue
		}
		if seen[field.Name] {
			v.addError("DUPLICATE_STATE_FIELD", fmt.Sprintf("duplicate state field name: %s", field.Name), "", "", "state.fields")
			continue
		}
		seen[field.Name] = true

		// Validate schema
		v.validateStrongTyping("", fmt.Sprintf("state.fields[%d].schema", i), field.Schema)

		// Validate reducer compatibility
		v.validateReducerCompat(field, i)
	}
}

func (v *validator) validateReducerCompat(field model.StateField, idx int) {
	var s map[string]any
	if err := json.Unmarshal(field.Schema, &s); err != nil {
		return // schema error already caught
	}
	typ, _ := s["type"].(string)

	switch field.Reducer {
	case model.ReducerAppend:
		if typ != "array" {
			v.addError("REDUCER_INCOMPATIBLE", fmt.Sprintf("state field %q uses 'append' reducer but schema type is %q (must be 'array')", field.Name, typ), "", "", fmt.Sprintf("state.fields[%d].reducer", idx))
		}
	case model.ReducerMerge:
		if typ != "object" {
			v.addError("REDUCER_INCOMPATIBLE", fmt.Sprintf("state field %q uses 'merge' reducer but schema type is %q (must be 'object')", field.Name, typ), "", "", fmt.Sprintf("state.fields[%d].reducer", idx))
		}
	case model.ReducerReplace:
		// replace works with any type
	}
}

func (v *validator) validateStateBindings() {
	stateFields := make(map[string]bool)
	if v.graph.State != nil {
		for _, f := range v.graph.State.Fields {
			stateFields[f.Name] = true
		}
	}

	for _, node := range v.graph.Nodes {
		for _, sr := range node.StateReads {
			if !stateFields[sr.StateField] {
				v.addError("INVALID_STATE_REF", fmt.Sprintf("state read references non-existent state field %q", sr.StateField), node.ID, "", "state_reads")
			}
			if !hasPort(node.InputPorts, sr.Port) {
				v.addError("INVALID_STATE_PORT", fmt.Sprintf("state read maps to non-existent input port %q", sr.Port), node.ID, "", "state_reads")
			}
		}
		for _, sw := range node.StateWrites {
			if !stateFields[sw.StateField] {
				v.addError("INVALID_STATE_REF", fmt.Sprintf("state write references non-existent state field %q", sw.StateField), node.ID, "", "state_writes")
			}
			if !hasPort(node.OutputPorts, sw.Port) {
				v.addError("INVALID_STATE_PORT", fmt.Sprintf("state write maps to non-existent output port %q", sw.Port), node.ID, "", "state_writes")
			}
		}
	}
}

func (v *validator) validateBackEdges() {
	for _, edge := range v.graph.Edges {
		if !edge.BackEdge {
			continue
		}
		if edge.Condition == "" {
			v.addError("BACKEDGE_NO_CONDITION", "back-edge must have a condition expression", "", edge.ID, "condition")
		}
		if edge.MaxIterations == nil || *edge.MaxIterations <= 0 {
			v.addError("BACKEDGE_NO_MAX_ITERATIONS", "back-edge must have max_iterations > 0", "", edge.ID, "max_iterations")
		}
	}
}

func (v *validator) validateNoCycles() {
	// Build adjacency list excluding back-edges
	adj := make(map[string][]string)
	for _, edge := range v.graph.Edges {
		if !edge.BackEdge {
			adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
		}
	}

	// DFS cycle detection
	white := 0 // not visited
	gray := 1  // in progress
	black := 2 // done
	colors := make(map[string]int)
	for _, node := range v.graph.Nodes {
		colors[node.ID] = white
	}

	var hasCycle bool
	var cyclePath []string

	var dfs func(nodeID string) bool
	dfs = func(nodeID string) bool {
		colors[nodeID] = gray
		cyclePath = append(cyclePath, nodeID)
		for _, neighbor := range adj[nodeID] {
			if colors[neighbor] == gray {
				hasCycle = true
				return true
			}
			if colors[neighbor] == white {
				if dfs(neighbor) {
					return true
				}
			}
		}
		cyclePath = cyclePath[:len(cyclePath)-1]
		colors[nodeID] = black
		return false
	}

	for _, node := range v.graph.Nodes {
		if colors[node.ID] == white {
			dfs(node.ID)
		}
	}

	if hasCycle {
		v.addError("UNGUARDED_CYCLE", fmt.Sprintf("graph contains a cycle not guarded by a back-edge (nodes: %s)", strings.Join(cyclePath, " → ")), "", "", "edges")
	}
}

func (v *validator) validateExclusiveFanIn() {
	// Group edges by target port (target_node_id + target_port)
	type portKey struct {
		nodeID string
		port   string
	}
	portEdges := make(map[portKey][]string) // port → edge IDs
	for _, edge := range v.graph.Edges {
		key := portKey{edge.TargetNodeID, edge.TargetPort}
		portEdges[key] = append(portEdges[key], edge.ID)
	}

	for key, edgeIDs := range portEdges {
		if len(edgeIDs) > 1 {
			// Multiple edges to the same port — must be from exclusive conditional branches
			// For now, emit a warning (full exclusivity analysis requires tracing conditional ancestry)
			v.addWarning("MULTI_EDGE_FAN_IN",
				fmt.Sprintf("port %s.%s has %d incoming edges — ensure they are from mutually exclusive conditional branches", key.nodeID, key.port, len(edgeIDs)),
				key.nodeID)
		}
	}
}

func (v *validator) validateReachability() {
	// BFS from input nodes
	reachable := make(map[string]bool)
	var queue []string
	for _, node := range v.graph.Nodes {
		if node.Type == model.NodeTypeInput {
			queue = append(queue, node.ID)
			reachable[node.ID] = true
		}
	}

	// Build adjacency (all edges, including back-edges)
	adj := make(map[string][]string)
	for _, edge := range v.graph.Edges {
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbor := range adj[current] {
			if !reachable[neighbor] {
				reachable[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	for _, node := range v.graph.Nodes {
		if !reachable[node.ID] {
			v.addWarning("UNREACHABLE_NODE",
				fmt.Sprintf("node %q is not reachable from any input node", node.ID),
				node.ID)
		}
	}
}

func (v *validator) validateRequiredPorts() {
	// Build set of wired input ports (from edges and state reads)
	type portKey struct {
		nodeID string
		port   string
	}
	wired := make(map[portKey]bool)

	for _, edge := range v.graph.Edges {
		wired[portKey{edge.TargetNodeID, edge.TargetPort}] = true
	}
	for _, node := range v.graph.Nodes {
		for _, sr := range node.StateReads {
			wired[portKey{node.ID, sr.Port}] = true
		}
	}

	// Check each required input port
	for _, node := range v.graph.Nodes {
		if node.Type == model.NodeTypeInput {
			continue // input nodes have no input ports to wire
		}
		for _, port := range node.InputPorts {
			if !port.IsRequired() {
				continue
			}
			if port.Default != nil {
				continue // has a default value
			}
			if !wired[portKey{node.ID, port.Name}] {
				v.addError("UNWIRED_REQUIRED_PORT",
					fmt.Sprintf("required input port %q on node %q is not wired (no edge, state read, or default)", port.Name, node.ID),
					node.ID, "", "input_ports")
			}
		}
	}
}

func (v *validator) validateToolLoopConfigs() {
	for i := range v.graph.Nodes {
		node := &v.graph.Nodes[i]
		if node.Type != model.NodeTypeLLM {
			continue
		}

		var cfg model.LLMNodeConfig
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			continue // Config parsing error will be caught at execution time
		}

		if !cfg.ToolLoop {
			continue
		}

		// Tool routing required (explicit routing, state, input, or API tool refs)
		if len(cfg.ToolRouting) == 0 && cfg.ToolRoutingFromState == "" && !cfg.ToolRoutingFromInput && len(cfg.APITools) == 0 {
			v.addError("TOOL_LOOP_NO_ROUTING",
				"tool_loop requires tool_routing, tool_routing_from_state, tool_routing_from_input, or api_tools",
				node.ID, "", "config.tool_routing")
		}

		// Validate API tool refs
		for j, ref := range cfg.APITools {
			if ref.APIToolID == "" {
				v.addError("API_TOOL_REF_NO_ID",
					fmt.Sprintf("api_tools[%d] requires 'api_tool_id'", j),
					node.ID, "", fmt.Sprintf("config.api_tools[%d].api_tool_id", j))
			}
			if ref.Endpoint == "" {
				v.addError("API_TOOL_REF_NO_ENDPOINT",
					fmt.Sprintf("api_tools[%d] requires 'endpoint'", j),
					node.ID, "", fmt.Sprintf("config.api_tools[%d].endpoint", j))
			}
		}

		// Tool routing from input requires the port
		if cfg.ToolRoutingFromInput && !hasPort(node.InputPorts, "tool_routing") {
			v.addError("TOOL_ROUTING_INPUT_PORT_MISSING",
				"tool_routing_from_input requires an input port named 'tool_routing'",
				node.ID, "", "config.tool_routing_from_input")
		}

		// Tool definitions without matching routing (warning only)
		if len(cfg.Tools) > 0 && len(cfg.ToolRouting) > 0 {
			for _, tool := range cfg.Tools {
				if _, ok := cfg.ToolRouting[tool.Name]; !ok {
					v.addWarning("TOOL_NO_ROUTING",
						fmt.Sprintf("tool %q has a definition but no routing — LLM will see it but calls will fail", tool.Name),
						node.ID)
				}
			}
		}

		// Routing entries must have exactly one target (MCP or API)
		for name, route := range cfg.ToolRouting {
			hasMCP := route.MCPURL != ""
			hasAPI := route.APIToolID != "" || route.APIEndpoint != ""
			if !hasMCP && !hasAPI {
				v.addError("TOOL_ROUTE_NO_TARGET",
					fmt.Sprintf("tool_routing entry %q has no mcp_url or api_tool_id", name),
					node.ID, "", "config.tool_routing."+name)
			}
			if hasMCP && hasAPI {
				v.addError("TOOL_ROUTE_AMBIGUOUS",
					fmt.Sprintf("tool_routing entry %q has both mcp_url and api_tool_id — exactly one required", name),
					node.ID, "", "config.tool_routing."+name)
			}
			if route.APIToolID != "" && route.APIEndpoint == "" {
				v.addError("TOOL_ROUTE_INCOMPLETE",
					fmt.Sprintf("tool_routing entry %q has api_tool_id but no api_endpoint", name),
					node.ID, "", "config.tool_routing."+name+".api_endpoint")
			}
			if route.TimeoutSeconds != nil && *route.TimeoutSeconds <= 0 {
				v.addError("TOOL_ROUTE_BAD_TIMEOUT",
					fmt.Sprintf("tool_routing entry %q timeout_seconds must be positive", name),
					node.ID, "", "config.tool_routing."+name+".timeout_seconds")
			}
			if route.Compacted && route.MCPURL == "" {
				v.addError("TOOL_ROUTE_COMPACTED_NO_MCP",
					fmt.Sprintf("tool_routing entry %q has compacted=true but no mcp_url", name),
					node.ID, "", "config.tool_routing."+name)
			}
		}

		// Warn if compacted routes exist but no system_prompt provides context.
		hasCompactedRoute := false
		for _, route := range cfg.ToolRouting {
			if route.Compacted {
				hasCompactedRoute = true
				break
			}
		}
		if hasCompactedRoute && cfg.SystemPrompt == "" {
			v.addWarning("TOOL_COMPACTED_NO_CONTEXT",
				"compacted MCP route(s) present but no system_prompt — LLM has no context about the MCP server capabilities",
				node.ID)
		}

		// Limits must be positive
		if cfg.MaxToolCalls != nil && *cfg.MaxToolCalls <= 0 {
			v.addError("TOOL_LOOP_BAD_LIMIT",
				"max_tool_calls must be positive",
				node.ID, "", "config.max_tool_calls")
		}
		if cfg.MaxLoopIterations != nil && *cfg.MaxLoopIterations <= 0 {
			v.addError("TOOL_LOOP_BAD_LIMIT",
				"max_loop_iterations must be positive",
				node.ID, "", "config.max_loop_iterations")
		}

		// Tool choice must be valid
		if cfg.ToolChoice != "" {
			validChoices := map[string]bool{"auto": true, "none": true, "required": true}
			if !validChoices[cfg.ToolChoice] {
				// Check if it's a tool name
				found := false
				for _, tool := range cfg.Tools {
					if tool.Name == cfg.ToolChoice {
						found = true
						break
					}
				}
				if !found {
					v.addError("TOOL_CHOICE_INVALID",
						fmt.Sprintf("invalid tool_choice: %q", cfg.ToolChoice),
						node.ID, "", "config.tool_choice")
				}
			}
		}
	}
}

func hasPort(ports []model.Port, name string) bool {
	for _, p := range ports {
		if p.Name == name {
			return true
		}
	}
	return false
}

// templateVarRegex matches {{input.XXX}} patterns in prompt templates.
var templateVarRegex = regexp.MustCompile(`\{\{\s*input\.(\w+)\s*\}\}`)

func (v *validator) validateSuperagentConfigs() {
	for i := range v.graph.Nodes {
		node := &v.graph.Nodes[i]
		if node.Type != model.NodeTypeSuperagent {
			continue
		}

		var cfg model.SuperagentNodeConfig
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			v.addError("SUPERAGENT_MISSING_CONFIG",
				fmt.Sprintf("superagent node has invalid config JSON: %v", err),
				node.ID, "", "config")
			continue
		}

		// Rule 1: Required config fields.
		if cfg.Prompt == "" {
			v.addError("SUPERAGENT_MISSING_CONFIG", "superagent node requires 'prompt'", node.ID, "", "config.prompt")
		}
		if len(cfg.Skills) == 0 {
			v.addError("SUPERAGENT_MISSING_CONFIG", "superagent node requires at least one skill", node.ID, "", "config.skills")
		}
		if cfg.Provider == "" {
			v.addError("SUPERAGENT_MISSING_CONFIG", "superagent node requires 'provider'", node.ID, "", "config.provider")
		}
		if cfg.Model == "" {
			v.addError("SUPERAGENT_MISSING_CONFIG", "superagent node requires 'model'", node.ID, "", "config.model")
		}

		// Rule 2: Skill validation.
		for j, skill := range cfg.Skills {
			if skill.Name == "" {
				v.addError("SUPERAGENT_INVALID_SKILL", fmt.Sprintf("skill[%d] requires 'name'", j), node.ID, "", fmt.Sprintf("config.skills[%d].name", j))
			}
			if skill.Description == "" {
				v.addError("SUPERAGENT_INVALID_SKILL", fmt.Sprintf("skill[%d] requires 'description'", j), node.ID, "", fmt.Sprintf("config.skills[%d].description", j))
			}
			// Exactly one of mcp_url or api_tool_id must be set
			hasMCP := skill.MCPURL != ""
			hasAPI := skill.APIToolID != ""
			if !hasMCP && !hasAPI {
				v.addError("SUPERAGENT_INVALID_SKILL", fmt.Sprintf("skill[%d] requires 'mcp_url' or 'api_tool_id'", j), node.ID, "", fmt.Sprintf("config.skills[%d]", j))
			}
			if hasMCP && hasAPI {
				v.addError("SUPERAGENT_INVALID_SKILL", fmt.Sprintf("skill[%d] has both 'mcp_url' and 'api_tool_id' — exactly one required", j), node.ID, "", fmt.Sprintf("config.skills[%d]", j))
			}
			if hasAPI && len(skill.Endpoints) == 0 {
				v.addError("SUPERAGENT_API_SKILL_NO_ENDPOINTS", fmt.Sprintf("skill[%d] with api_tool_id requires non-empty 'endpoints'", j), node.ID, "", fmt.Sprintf("config.skills[%d].endpoints", j))
			}
			if skill.Compacted && !hasMCP {
				v.addError("SUPERAGENT_COMPACTED_NO_MCP", fmt.Sprintf("skill[%d] has compacted=true but no mcp_url", j), node.ID, "", fmt.Sprintf("config.skills[%d]", j))
			}
			if skill.Compacted && skill.Description == "" && len(skill.Tools) == 0 {
				v.addWarning("SUPERAGENT_COMPACTED_NO_CONTEXT", fmt.Sprintf("skill[%d] has compacted=true but no description or tools — LLM has no context about the MCP server", j), node.ID)
			}
		}

		// Rule 3: API key.
		if cfg.APIKey == "" && cfg.APIKeyRef == "" {
			v.addError("SUPERAGENT_MISSING_CONFIG", "superagent node requires 'api_key' or 'api_key_ref'", node.ID, "", "config.api_key")
		}

		// Rule 3b: Numeric limit range checks (must be > 0 when set).
		if cfg.MaxIterations != nil && *cfg.MaxIterations <= 0 {
			v.addError("SUPERAGENT_MISSING_CONFIG", "max_iterations must be > 0", node.ID, "", "config.max_iterations")
		}
		if cfg.MaxTotalToolCalls != nil && *cfg.MaxTotalToolCalls <= 0 {
			v.addError("SUPERAGENT_MISSING_CONFIG", "max_total_tool_calls must be > 0", node.ID, "", "config.max_total_tool_calls")
		}
		if cfg.MaxToolCallsPerIteration != nil && *cfg.MaxToolCallsPerIteration <= 0 {
			v.addError("SUPERAGENT_MISSING_CONFIG", "max_tool_calls_per_iteration must be > 0", node.ID, "", "config.max_tool_calls_per_iteration")
		}
		if cfg.TimeoutSeconds != nil && *cfg.TimeoutSeconds <= 0 {
			v.addError("SUPERAGENT_MISSING_CONFIG", "timeout_seconds must be > 0", node.ID, "", "config.timeout_seconds")
		}

		// Rule 4: At least one output port.
		if len(node.OutputPorts) == 0 {
			v.addError("SUPERAGENT_NO_OUTPUT", "superagent node requires at least one output port", node.ID, "", "output_ports")
		}

		// Rule 5: Shared memory state field.
		if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled {
			// Graph must have _superagent_memory state field.
			found := false
			if v.graph.State != nil {
				for _, f := range v.graph.State.Fields {
					if f.Name == "_superagent_memory" && f.Reducer == model.ReducerMerge {
						found = true
						break
					}
				}
			}
			if !found {
				v.addError("SUPERAGENT_MISSING_SHARED_MEMORY_STATE",
					"shared_memory.enabled requires graph state field '_superagent_memory' with merge reducer",
					node.ID, "", "config.shared_memory")
			}

			// Node must have state reads/writes for _superagent_memory.
			hasRead := false
			hasWrite := false
			for _, sr := range node.StateReads {
				if sr.StateField == "_superagent_memory" {
					hasRead = true
				}
			}
			for _, sw := range node.StateWrites {
				if sw.StateField == "_superagent_memory" {
					hasWrite = true
				}
			}
			if !hasRead || !hasWrite {
				v.addError("SUPERAGENT_MISSING_SHARED_MEMORY_STATE",
					"shared_memory.enabled requires state_reads and state_writes for '_superagent_memory'",
					node.ID, "", "state_reads/state_writes")
			}
		}

		// Rule 6: Conversation history input port.
		if cfg.ConversationHistoryFromInput != "" {
			if !hasPort(node.InputPorts, cfg.ConversationHistoryFromInput) {
				v.addError("SUPERAGENT_MISSING_CONFIG",
					fmt.Sprintf("conversation_history_from_input references non-existent input port %q", cfg.ConversationHistoryFromInput),
					node.ID, "", "config.conversation_history_from_input")
			}
		}

		// Rule 7: Override consistency.
		v.validateSuperagentOverrides(node.ID, cfg.Provider, cfg.Overrides)

		// Rule 8: Template variables (warning).
		v.validateSuperagentTemplateVars(node.ID, cfg.Prompt, node.InputPorts)

		// Rule 9: Code execution config.
		v.validateCodeExecutionConfig(node.ID, cfg.CodeExecution)
	}
}

func (v *validator) validateCodeExecutionConfig(nodeID string, ce *model.CodeExecutionConfig) {
	if ce == nil || !ce.Enabled {
		return
	}

	if ce.MaxExecutionTimeSec != nil {
		if *ce.MaxExecutionTimeSec <= 0 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_execution_time_sec must be > 0", nodeID, "", "config.code_execution.max_execution_time_sec")
		} else if *ce.MaxExecutionTimeSec > 300 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_execution_time_sec must be <= 300", nodeID, "", "config.code_execution.max_execution_time_sec")
		}
	}
	if ce.MaxMemoryMB != nil {
		if *ce.MaxMemoryMB <= 0 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_memory_mb must be > 0", nodeID, "", "config.code_execution.max_memory_mb")
		} else if *ce.MaxMemoryMB > 2048 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_memory_mb must be <= 2048", nodeID, "", "config.code_execution.max_memory_mb")
		}
	}
	if ce.MaxOutputBytes != nil {
		if *ce.MaxOutputBytes <= 0 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_output_bytes must be > 0", nodeID, "", "config.code_execution.max_output_bytes")
		} else if *ce.MaxOutputBytes > 10*1048576 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_output_bytes must be <= 10MB", nodeID, "", "config.code_execution.max_output_bytes")
		}
	}
	if ce.MaxCodeBytes != nil {
		if *ce.MaxCodeBytes <= 0 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_code_bytes must be > 0", nodeID, "", "config.code_execution.max_code_bytes")
		} else if *ce.MaxCodeBytes > 1048576 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_code_bytes must be <= 1MB", nodeID, "", "config.code_execution.max_code_bytes")
		}
	}
	if ce.MaxToolCallsPerExecution != nil {
		if *ce.MaxToolCallsPerExecution <= 0 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_tool_calls_per_execution must be > 0", nodeID, "", "config.code_execution.max_tool_calls_per_execution")
		} else if *ce.MaxToolCallsPerExecution > 500 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_tool_calls_per_execution must be <= 500", nodeID, "", "config.code_execution.max_tool_calls_per_execution")
		}
	}
	if ce.MaxExecutionsPerRun != nil {
		if *ce.MaxExecutionsPerRun <= 0 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_executions_per_run must be > 0", nodeID, "", "config.code_execution.max_executions_per_run")
		} else if *ce.MaxExecutionsPerRun > 100 {
			v.addError("SUPERAGENT_INVALID_CODE_EXEC", "code_execution.max_executions_per_run must be <= 100", nodeID, "", "config.code_execution.max_executions_per_run")
		}
	}
}

func (v *validator) validateSuperagentOverrides(nodeID string, mainProvider model.ProviderType, overrides *model.SuperagentOverrides) {
	if overrides == nil {
		return
	}

	// Helper: if provider is set, model must also be set.
	checkProviderModel := func(name string, provider model.ProviderType, modelName, apiKey, apiKeyRef string) {
		if provider != "" && modelName == "" {
			v.addError("SUPERAGENT_INVALID_OVERRIDE",
				fmt.Sprintf("%s override has provider but no model", name),
				nodeID, "", "config.overrides."+name)
		}
		// Warn if override uses a different provider without its own credentials.
		if provider != "" && provider != mainProvider && apiKey == "" && apiKeyRef == "" {
			v.addWarning("SUPERAGENT_INVALID_OVERRIDE",
				fmt.Sprintf("%s override uses provider %q (differs from main %q) but has no api_key or api_key_ref", name, provider, mainProvider),
				nodeID)
		}
	}

	if overrides.Evaluator != nil {
		checkProviderModel("evaluator", overrides.Evaluator.Provider, overrides.Evaluator.Model, overrides.Evaluator.APIKey, overrides.Evaluator.APIKeyRef)
	}
	if overrides.Reflection != nil {
		checkProviderModel("reflection", overrides.Reflection.Provider, overrides.Reflection.Model, overrides.Reflection.APIKey, overrides.Reflection.APIKeyRef)
	}
	if overrides.ContextCompaction != nil {
		checkProviderModel("context_compaction", overrides.ContextCompaction.Provider, overrides.ContextCompaction.Model, overrides.ContextCompaction.APIKey, overrides.ContextCompaction.APIKeyRef)
	}
	if overrides.OutputExtraction != nil {
		checkProviderModel("output_extraction", overrides.OutputExtraction.Provider, overrides.OutputExtraction.Model, overrides.OutputExtraction.APIKey, overrides.OutputExtraction.APIKeyRef)
	}

	// Stuck detection: window_size must be > 0 (prevents divide-by-zero panic).
	if overrides.StuckDetection != nil && overrides.StuckDetection.WindowSize != nil && *overrides.StuckDetection.WindowSize <= 0 {
		v.addError("SUPERAGENT_INVALID_OVERRIDE",
			"stuck_detection.window_size must be > 0",
			nodeID, "", "config.overrides.stuck_detection.window_size")
	}

	// Compaction threshold must be in (0.0, 1.0].
	if overrides.ContextCompaction != nil && overrides.ContextCompaction.CompactionThreshold != nil {
		t := *overrides.ContextCompaction.CompactionThreshold
		if t <= 0 || t > 1.0 {
			v.addError("SUPERAGENT_INVALID_OVERRIDE",
				"context_compaction.compaction_threshold must be in range (0.0, 1.0]",
				nodeID, "", "config.overrides.context_compaction.compaction_threshold")
		}
	}
}

func (v *validator) validateSuperagentTemplateVars(nodeID, prompt string, inputPorts []model.Port) {
	matches := templateVarRegex.FindAllStringSubmatch(prompt, -1)
	for _, match := range matches {
		varName := match[1]
		if !hasPort(inputPorts, varName) {
			v.addWarning("SUPERAGENT_TEMPLATE_VAR_MISSING",
				fmt.Sprintf("prompt references {{input.%s}} but no input port %q is declared", varName, varName),
				nodeID)
		}
	}
}

func (v *validator) validateAPIToolNodeConfigs() {
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	}
	for i := range v.graph.Nodes {
		node := &v.graph.Nodes[i]
		if node.Type != model.NodeTypeAPITool {
			continue
		}

		var cfg model.APIToolNodeConfig
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			v.addError("API_TOOL_INVALID_CONFIG",
				fmt.Sprintf("api_tool node has invalid config JSON: %v", err),
				node.ID, "", "config")
			continue
		}

		hasRef := cfg.APIToolID != "" || cfg.Endpoint != ""
		hasInline := cfg.InlineEndpoint != nil

		if !hasRef && !hasInline {
			v.addError("API_TOOL_NO_DEFINITION",
				"api_tool node requires either api_tool_id+endpoint or inline_endpoint",
				node.ID, "", "config")
			continue
		}
		if hasRef && hasInline {
			v.addError("API_TOOL_AMBIGUOUS",
				"api_tool node has both api_tool_id and inline_endpoint — exactly one required",
				node.ID, "", "config")
		}

		if hasRef {
			if cfg.APIToolID == "" {
				v.addError("API_TOOL_REF_NO_ID", "api_tool node requires 'api_tool_id'", node.ID, "", "config.api_tool_id")
			}
			if cfg.Endpoint == "" {
				v.addError("API_TOOL_REF_NO_ENDPOINT", "api_tool node requires 'endpoint'", node.ID, "", "config.endpoint")
			}
		}

		if hasInline {
			ep := cfg.InlineEndpoint
			if ep.BaseURL == "" {
				v.addError("API_TOOL_INLINE_NO_BASE_URL", "inline_endpoint requires 'base_url'", node.ID, "", "config.inline_endpoint.base_url")
			} else if !strings.HasPrefix(ep.BaseURL, "http://") && !strings.HasPrefix(ep.BaseURL, "https://") {
				v.addError("API_TOOL_INLINE_BAD_URL", "inline_endpoint base_url must start with http:// or https://", node.ID, "", "config.inline_endpoint.base_url")
			}
			if ep.Method == "" {
				v.addError("API_TOOL_INLINE_NO_METHOD", "inline_endpoint requires 'method'", node.ID, "", "config.inline_endpoint.method")
			} else if !validMethods[strings.ToUpper(ep.Method)] {
				v.addError("API_TOOL_INLINE_BAD_METHOD",
					fmt.Sprintf("inline_endpoint method %q is not valid (use GET, POST, PUT, PATCH, DELETE)", ep.Method),
					node.ID, "", "config.inline_endpoint.method")
			}
			if ep.Path == "" {
				v.addError("API_TOOL_INLINE_NO_PATH", "inline_endpoint requires 'path'", node.ID, "", "config.inline_endpoint.path")
			}
		}
	}
}
