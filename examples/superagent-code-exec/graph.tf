# E2E Superagent Code Execution
#
# Terraform representation of the E2E Superagent Code Execution test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E Superagent Code Execution"
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
          "name": "data",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {}
    },
    {
      "id": "agent-1",
      "name": "Code Agent",
      "type": "superagent",
      "input_ports": [
        {
          "name": "data",
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
        "prompt": "Transform the input data using code execution: {{input.data}}",
        "provider": "openrouter",
        "model": "superagent-code-exec",
        "base_url": "http://mock-llm:9091/v1",
        "api_key": "mock-key",
        "max_iterations": 3,
        "max_total_tool_calls": 10,
        "temperature": 0,
        "timeout_seconds": 60,
        "skills": [
          {
            "name": "tools",
            "description": "General tools",
            "mcp_url": "http://mcp-test-server:9090"
          }
        ],
        "code_execution": {
          "enabled": true,
          "max_execution_time_sec": 10,
          "max_executions_per_run": 5
        },
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
      "source_port": "data",
      "target_node_id": "agent-1",
      "target_port": "data"
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
