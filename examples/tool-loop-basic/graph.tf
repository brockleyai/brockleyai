# E2E Tool Loop Basic
#
# Terraform representation of the E2E Tool Loop Basic test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E Tool Loop Basic"
}

resource "brockley_graph" "test" {
  name        = var.name
  namespace   = "e2e-tests"
  description = "E2E test: single LLM node with tool_loop calling echo and word_count tools."

  nodes = <<-JSON
  [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [
        {
          "name": "user_message",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {}
    },
    {
      "id": "assistant",
      "name": "Assistant",
      "type": "llm",
      "input_ports": [
        {
          "name": "user_prompt",
          "schema": {
            "type": "string"
          }
        }
      ],
      "output_ports": [
        {
          "name": "response_text",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "finish_reason",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "total_tool_calls",
          "schema": {
            "type": "integer"
          }
        },
        {
          "name": "iterations",
          "schema": {
            "type": "integer"
          }
        }
      ],
      "config": {
        "provider": "openai",
        "model": "tool-loop-basic",
        "base_url": "http://mock-llm:9091",
        "api_key": "test-key",
        "user_prompt": "{{input.user_prompt}}",
        "tool_loop": true,
        "tool_routing": {
          "echo": {
            "mcp_url": "http://mcp-test-server:9090"
          },
          "word_count": {
            "mcp_url": "http://mcp-test-server:9090"
          }
        }
      }
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [
        {
          "name": "response",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "finish_reason",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "total_tool_calls",
          "schema": {
            "type": "integer"
          }
        },
        {
          "name": "iterations",
          "schema": {
            "type": "integer"
          }
        }
      ],
      "output_ports": [
        {
          "name": "response",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "finish_reason",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "total_tool_calls",
          "schema": {
            "type": "integer"
          }
        },
        {
          "name": "iterations",
          "schema": {
            "type": "integer"
          }
        }
      ],
      "config": {}
    }
  ]
  JSON

  edges = <<-JSON
  [
    {
      "id": "e1",
      "source_node_id": "input-1",
      "source_port": "user_message",
      "target_node_id": "assistant",
      "target_port": "user_prompt"
    },
    {
      "id": "e2",
      "source_node_id": "assistant",
      "source_port": "response_text",
      "target_node_id": "output-1",
      "target_port": "response"
    },
    {
      "id": "e3",
      "source_node_id": "assistant",
      "source_port": "finish_reason",
      "target_node_id": "output-1",
      "target_port": "finish_reason"
    },
    {
      "id": "e4",
      "source_node_id": "assistant",
      "source_port": "total_tool_calls",
      "target_node_id": "output-1",
      "target_port": "total_tool_calls"
    },
    {
      "id": "e5",
      "source_node_id": "assistant",
      "source_port": "iterations",
      "target_node_id": "output-1",
      "target_port": "iterations"
    }
  ]
  JSON
}
