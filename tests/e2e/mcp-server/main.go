package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// jsonRPCRequest is a JSON-RPC 2.0 request envelope.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response envelope.
type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

// contentBlock is an MCP content block.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// toolCallResult is the result of a tools/call invocation.
type toolCallResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

var verbose bool

func main() {
	port := "9090"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	verbose = os.Getenv("VERBOSE") == "true"

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/", jsonRPCHandler)

	log.Printf("MCP test server listening on :%s (verbose=%v)", port, verbose)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func jsonRPCHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONRPCError(w, nil, -32700, "parse error")
		return
	}

	if verbose {
		log.Printf("[VERBOSE] <-- %s %s body=%s", r.Method, r.URL.Path, string(body))
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRPCError(w, nil, -32700, "parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		writeJSONRPCError(w, req.ID, -32600, "invalid request: jsonrpc must be 2.0")
		return
	}

	switch req.Method {
	case "tools/list":
		handleToolsList(w, req)
	case "tools/call":
		handleToolsCall(w, r, req)
	default:
		writeJSONRPCError(w, req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func handleToolsList(w http.ResponseWriter, req jsonRPCRequest) {
	result := map[string]any{
		"tools": getToolDefinitions(),
	}
	writeJSONRPCResult(w, req.ID, result)
}

func handleToolsCall(w http.ResponseWriter, r *http.Request, req jsonRPCRequest) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	if verbose {
		argsJSON, _ := json.Marshal(params.Arguments)
		log.Printf("[VERBOSE] tool call: %s args=%s", params.Name, string(argsJSON))
	}

	result := dispatchTool(params.Name, params.Arguments, r.Header)

	if verbose {
		resultJSON, _ := json.Marshal(result)
		log.Printf("[VERBOSE] --> tool result: %s is_error=%v result=%s", params.Name, result.IsError, string(resultJSON))
	}

	writeJSONRPCResult(w, req.ID, result)
}

func writeJSONRPCResult(w http.ResponseWriter, id any, result any) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeJSONRPCError(w http.ResponseWriter, id any, code int, message string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]any{
			"code":    code,
			"message": message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
