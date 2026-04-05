# E2E Tool Loop Two Nodes
#
# Terraform representation of the E2E Tool Loop Two Nodes test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E Tool Loop Two Nodes"
}

resource "brockley_graph" "test" {
  name        = var.name
  namespace   = "e2e-tests"
  description = "E2E test: two separate LLM nodes with tool loops wired sequentially."

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
      "id": "llm-1",
      "name": "First Assistant",
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
        "model": "tool-loop-two-nodes-1",
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
      "id": "llm-2",
      "name": "Second Assistant",
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
        "model": "tool-loop-two-nodes-2",
        "base_url": "http://mock-llm:9091",
        "api_key": "test-key",
        "user_prompt": "{{input.user_prompt}}",
        "tool_loop": true,
        "tool_routing": {
          "transform_text": {
            "mcp_url": "http://mcp-test-server:9090"
          },
          "store_value": {
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
          "name": "first_response",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "second_response",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "first_tool_calls",
          "schema": {
            "type": "integer"
          }
        },
        {
          "name": "second_tool_calls",
          "schema": {
            "type": "integer"
          }
        }
      ],
      "output_ports": [
        {
          "name": "first_response",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "second_response",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "first_tool_calls",
          "schema": {
            "type": "integer"
          }
        },
        {
          "name": "second_tool_calls",
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
      "target_node_id": "llm-1",
      "target_port": "user_prompt"
    },
    {
      "id": "e2",
      "source_node_id": "llm-1",
      "source_port": "response_text",
      "target_node_id": "llm-2",
      "target_port": "user_prompt"
    },
    {
      "id": "e3",
      "source_node_id": "llm-1",
      "source_port": "response_text",
      "target_node_id": "output-1",
      "target_port": "first_response"
    },
    {
      "id": "e4",
      "source_node_id": "llm-2",
      "source_port": "response_text",
      "target_node_id": "output-1",
      "target_port": "second_response"
    },
    {
      "id": "e5",
      "source_node_id": "llm-1",
      "source_port": "total_tool_calls",
      "target_node_id": "output-1",
      "target_port": "first_tool_calls"
    },
    {
      "id": "e6",
      "source_node_id": "llm-2",
      "source_port": "total_tool_calls",
      "target_node_id": "output-1",
      "target_port": "second_tool_calls"
    }
  ]
  JSON
}
