// Package main implements a mock LLM provider HTTP server for E2E tool loop tests.
// It simulates tool-calling LLM responses with deterministic, scripted behavior.
// Configurable via a JSON script file that defines the response sequence per scenario.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

// ScriptedResponse defines a single response in a scenario.
type ScriptedResponse struct {
	Content      string     `json:"content,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"` // "stop" or "tool_calls"
}

// ToolCall matches the OpenAI tool_calls wire format.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Scenario maps a model name to a sequence of responses.
type Scenario struct {
	Responses []ScriptedResponse `json:"responses"`
}

var (
	scenarios   map[string]Scenario
	scenarioMu  sync.Mutex
	callIndices map[string]int
)

func main() {
	port := "9091"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	// Load script file if provided.
	scriptFile := os.Getenv("SCRIPT_FILE")
	if scriptFile == "" {
		scriptFile = "scripts.json"
	}

	scenarios = make(map[string]Scenario)
	callIndices = make(map[string]int)

	if data, err := os.ReadFile(scriptFile); err == nil {
		if err := json.Unmarshal(data, &scenarios); err != nil {
			log.Fatalf("Failed to parse script file %s: %v", scriptFile, err)
		}
		log.Printf("Loaded %d scenarios from %s", len(scenarios), scriptFile)
	} else {
		log.Printf("No script file found at %s, using default behavior", scriptFile)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("/v1/chat/completions", handleCompletion)
	mux.HandleFunc("/reset", handleReset)

	log.Printf("Mock LLM server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handleReset(w http.ResponseWriter, _ *http.Request) {
	scenarioMu.Lock()
	callIndices = make(map[string]int)
	scenarioMu.Unlock()
	fmt.Fprint(w, "ok")
}

// OpenAI-compatible request/response types.
type completionRequest struct {
	Model    string          `json:"model"`
	Messages []message       `json:"messages"`
	Tools    json.RawMessage `json:"tools,omitempty"`
}

type message struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type completionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
}

type choice struct {
	Index        int             `json:"index"`
	Message      responseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type responseMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func handleCompletion(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var req completionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "parse error", http.StatusBadRequest)
		return
	}

	scenarioMu.Lock()
	scenario, ok := scenarios[req.Model]
	if !ok {
		scenarioMu.Unlock()
		// Default: echo back a simple response
		writeDefaultResponse(w, req.Model)
		return
	}

	idx := callIndices[req.Model]
	callIndices[req.Model] = idx + 1
	scenarioMu.Unlock()

	if idx >= len(scenario.Responses) {
		writeDefaultResponse(w, req.Model)
		return
	}

	scripted := scenario.Responses[idx]
	resp := completionResponse{
		ID:     fmt.Sprintf("mock-%s-%d", req.Model, idx),
		Object: "chat.completion",
		Model:  req.Model,
		Choices: []choice{
			{
				Index: 0,
				Message: responseMessage{
					Role:      "assistant",
					Content:   scripted.Content,
					ToolCalls: scripted.ToolCalls,
				},
				FinishReason: scripted.FinishReason,
			},
		},
		Usage: usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeDefaultResponse(w http.ResponseWriter, model string) {
	resp := completionResponse{
		ID:     "mock-default",
		Object: "chat.completion",
		Model:  model,
		Choices: []choice{
			{
				Index: 0,
				Message: responseMessage{
					Role:    "assistant",
					Content: "I have completed the task.",
				},
				FinishReason: "stop",
			},
		},
		Usage: usage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
