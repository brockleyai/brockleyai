# E2E Tool Loop Chain
#
# Terraform representation of the E2E Tool Loop Chain test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E Tool Loop Chain"
}

resource "brockley_graph" "test" {
  name        = var.name
  namespace   = "e2e-tests"
  description = "E2E test: multi-step reasoning where tool results inform subsequent tool calls."

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
          "name": "total_tool_calls",
          "schema": {
            "type": "integer"
          }
        }
      ],
      "config": {
        "provider": "openai",
        "model": "tool-loop-chain",
        "base_url": "http://mock-llm:9091",
        "api_key": "test-key",
        "user_prompt": "{{input.user_prompt}}",
        "tool_loop": true,
        "tool_routing": {
          "store_value": {
            "mcp_url": "http://mcp-test-server:9090"
          },
          "retrieve_value": {
            "mcp_url": "http://mcp-test-server:9090"
          },
          "transform_text": {
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
          "name": "total_tool_calls",
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
          "name": "total_tool_calls",
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
      "source_port": "total_tool_calls",
      "target_node_id": "output-1",
      "target_port": "total_tool_calls"
    }
  ]
  JSON
}
