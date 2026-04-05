# E2E Superagent Compacted MCP
#
# Terraform representation of the E2E Superagent Compacted MCP test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E Superagent Compacted MCP"
}

variable "openrouter_api_key" {
  type      = string
  sensitive = true
  default   = ""
}

resource "brockley_graph" "test" {
  name        = var.name
  namespace   = "e2e-tests"

  nodes = <<-JSON
  [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [
        {
          "name": "message",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {}
    },
    {
      "id": "agent-1",
      "name": "Compacted Agent",
      "type": "superagent",
      "input_ports": [
        {
          "name": "message",
          "schema": {
            "type": "string"
          }
        }
      ],
      "output_ports": [
        {
          "name": "result",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {
        "prompt": "Use the echo tool to echo the message 'hello from compacted'. Reply with exactly the echo result.",
        "provider": "openrouter",
        "model": "openai/gpt-oss-20b",
        "api_key": "${var.openrouter_api_key}",
        "max_iterations": 3,
        "max_total_tool_calls": 5,
        "temperature": 0,
        "max_tokens": 128,
        "timeout_seconds": 60,
        "skills": [
          {
            "name": "text-tools",
            "description": "MCP server with text processing tools including echo, word_count, and lookup.",
            "mcp_url": "http://mcp-test-server:9090",
            "compacted": true,
            "tools": [
              "echo"
            ]
          }
        ],
        "overrides": {
          "evaluator": {
            "disabled": true
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
          "name": "result",
          "schema": {
            "type": "string"
          }
        }
      ],
      "output_ports": [
        {
          "name": "result",
          "schema": {
            "type": "string"
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
      "source_port": "message",
      "target_node_id": "agent-1",
      "target_port": "message"
    },
    {
      "id": "e2",
      "source_node_id": "agent-1",
      "source_port": "result",
      "target_node_id": "output-1",
      "target_port": "result"
    }
  ]
  JSON
}
