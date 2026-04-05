# E2E Superagent Oneshot
#
# Terraform representation of the E2E Superagent Oneshot test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E Superagent Oneshot"
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
          "name": "topic",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {}
    },
    {
      "id": "agent-1",
      "name": "Oneshot Agent",
      "type": "superagent",
      "input_ports": [
        {
          "name": "topic",
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
        "prompt": "Reply with exactly: hello from oneshot",
        "provider": "openrouter",
        "model": "openai/gpt-oss-20b",
        "api_key": "${var.openrouter_api_key}",
        "max_iterations": 1,
        "max_total_tool_calls": 5,
        "temperature": 0,
        "max_tokens": 128,
        "timeout_seconds": 60,
        "skills": [
          {
            "name": "tools",
            "description": "General tools",
            "mcp_url": "http://mcp-test-server:9090"
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
      "source_port": "topic",
      "target_node_id": "agent-1",
      "target_port": "topic"
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
