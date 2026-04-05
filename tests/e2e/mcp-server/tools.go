package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// keyValueStore is a simple in-memory KV store scoped per request session.
// For E2E tests, it's a global shared store (tests run sequentially).
var keyValueStore = struct {
	mu   sync.Mutex
	data map[string]string
}{data: make(map[string]string)}

// getToolDefinitions returns the MCP tool definitions for tools/list.
func getToolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "echo",
			"description": "Echoes the message back. Returns X-Test-Header value if present.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]any{"type": "string"},
				},
				"required": []string{"message"},
			},
		},
		{
			"name":        "word_count",
			"description": "Counts the number of words in the given text. Returns the count as a string.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{"type": "string"},
				},
				"required": []string{"text"},
			},
		},
		{
			"name":        "lookup",
			"description": "Looks up a value by key. Known keys: alpha, beta, gamma, delta.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{"type": "string"},
				},
				"required": []string{"key"},
			},
		},
		{
			"name":        "store_value",
			"description": "Stores a key/value pair in server-side memory. Returns 'stored: key=value'.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":   map[string]any{"type": "string"},
					"value": map[string]any{"type": "string"},
				},
				"required": []string{"key", "value"},
			},
		},
		{
			"name":        "retrieve_value",
			"description": "Retrieves a previously stored value by key. Returns the value or error if not found.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{"type": "string"},
				},
				"required": []string{"key"},
			},
		},
		{
			"name":        "transform_text",
			"description": "Applies a named transformation (uppercase, lowercase, reverse, word_sort) to input text.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"operation": map[string]any{"type": "string", "enum": []string{"uppercase", "lowercase", "reverse", "word_sort"}},
					"text":      map[string]any{"type": "string"},
				},
				"required": []string{"operation", "text"},
			},
		},
		{
			"name":        "multi_step_calc",
			"description": "Takes an operation (add, multiply, concat) and two operands. Returns the result.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"operation": map[string]any{"type": "string", "enum": []string{"add", "multiply", "concat"}},
					"a":         map[string]any{"type": "string"},
					"b":         map[string]any{"type": "string"},
				},
				"required": []string{"operation", "a", "b"},
			},
		},
	}
}

// lookupTable maps keys to ordinal values.
var lookupTable = map[string]string{
	"alpha": "first",
	"beta":  "second",
	"gamma": "third",
	"delta": "fourth",
}

// dispatchTool executes a tool by name and returns the MCP result.
func dispatchTool(name string, args map[string]any, headers http.Header) toolCallResult {
	switch name {
	case "echo":
		return toolEcho(args, headers)
	case "word_count":
		return toolWordCount(args)
	case "lookup":
		return toolLookup(args)
	case "store_value":
		return toolStoreValue(args)
	case "retrieve_value":
		return toolRetrieveValue(args)
	case "transform_text":
		return toolTransformText(args)
	case "multi_step_calc":
		return toolMultiStepCalc(args)
	default:
		return toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("unknown tool: %s", name)}},
			IsError: true,
		}
	}
}

func toolEcho(args map[string]any, headers http.Header) toolCallResult {
	message, _ := args["message"].(string)

	// If X-Test-Header is set, include it in the response.
	if h := headers.Get("X-Test-Header"); h != "" {
		message = fmt.Sprintf("[%s] %s", h, message)
	}

	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: message}},
	}
}

func toolWordCount(args map[string]any) toolCallResult {
	text, _ := args["text"].(string)
	words := strings.Fields(text)
	count := len(words)
	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("%d", count)}},
	}
}

func toolLookup(args map[string]any) toolCallResult {
	key, _ := args["key"].(string)
	value, ok := lookupTable[key]
	if !ok {
		return toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("unknown key: %s", key)}},
			IsError: true,
		}
	}
	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: value}},
	}
}

func toolStoreValue(args map[string]any) toolCallResult {
	key, _ := args["key"].(string)
	value, _ := args["value"].(string)
	keyValueStore.mu.Lock()
	keyValueStore.data[key] = value
	keyValueStore.mu.Unlock()
	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("stored: %s=%s", key, value)}},
	}
}

func toolRetrieveValue(args map[string]any) toolCallResult {
	key, _ := args["key"].(string)
	keyValueStore.mu.Lock()
	value, ok := keyValueStore.data[key]
	keyValueStore.mu.Unlock()
	if !ok {
		return toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("key not found: %s", key)}},
			IsError: true,
		}
	}
	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: value}},
	}
}

func toolTransformText(args map[string]any) toolCallResult {
	operation, _ := args["operation"].(string)
	text, _ := args["text"].(string)

	var result string
	switch operation {
	case "uppercase":
		result = strings.ToUpper(text)
	case "lowercase":
		result = strings.ToLower(text)
	case "reverse":
		runes := []rune(text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		result = string(runes)
	case "word_sort":
		words := strings.Fields(text)
		sort.Strings(words)
		result = strings.Join(words, " ")
	default:
		return toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("unknown operation: %s", operation)}},
			IsError: true,
		}
	}
	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: result}},
	}
}

func toolMultiStepCalc(args map[string]any) toolCallResult {
	operation, _ := args["operation"].(string)
	a, _ := args["a"].(string)
	b, _ := args["b"].(string)

	switch operation {
	case "add":
		aNum, err1 := strconv.ParseFloat(a, 64)
		bNum, err2 := strconv.ParseFloat(b, 64)
		if err1 != nil || err2 != nil {
			return toolCallResult{
				Content: []contentBlock{{Type: "text", Text: "operands must be numbers for add"}},
				IsError: true,
			}
		}
		return toolCallResult{
			Content: []contentBlock{{Type: "text", Text: strconv.FormatFloat(aNum+bNum, 'f', -1, 64)}},
		}
	case "multiply":
		aNum, err1 := strconv.ParseFloat(a, 64)
		bNum, err2 := strconv.ParseFloat(b, 64)
		if err1 != nil || err2 != nil {
			return toolCallResult{
				Content: []contentBlock{{Type: "text", Text: "operands must be numbers for multiply"}},
				IsError: true,
			}
		}
		return toolCallResult{
			Content: []contentBlock{{Type: "text", Text: strconv.FormatFloat(aNum*bNum, 'f', -1, 64)}},
		}
	case "concat":
		return toolCallResult{
			Content: []contentBlock{{Type: "text", Text: a + b}},
		}
	default:
		return toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("unknown operation: %s", operation)}},
			IsError: true,
		}
	}
}
