# E2E API Tool Standalone
#
# Terraform representation of the E2E API Tool Standalone test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E API Tool Standalone"
}

resource "brockley_graph" "test" {
  name        = var.name
  namespace   = "e2e-tests"
  description = "E2E test: standalone api_tool node calling a REST endpoint via referenced API tool definition."

  nodes = <<-JSON
  [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [
        {
          "name": "customer_id",
          "schema": {
            "type": "string"
          }
        }
      ],
      "output_ports": [
        {
          "name": "customer_id",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {},
      "position": {
        "x": 0,
        "y": 100
      }
    },
    {
      "id": "get-customer",
      "name": "Get Customer",
      "type": "api_tool",
      "input_ports": [
        {
          "name": "customer_id",
          "schema": {
            "type": "string"
          }
        }
      ],
      "output_ports": [
        {
          "name": "result",
          "schema": {
            "type": "object",
            "properties": {
              "id": {
                "type": "string"
              },
              "email": {
                "type": "string"
              },
              "name": {
                "type": "string"
              }
            }
          }
        }
      ],
      "config": {
        "api_tool_id": "test-api",
        "endpoint": "get_customer"
      },
      "position": {
        "x": 300,
        "y": 100
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
            "type": "object",
            "properties": {
              "id": {
                "type": "string"
              },
              "email": {
                "type": "string"
              },
              "name": {
                "type": "string"
              }
            }
          }
        }
      ],
      "output_ports": [
        {
          "name": "result",
          "schema": {
            "type": "object",
            "properties": {
              "id": {
                "type": "string"
              },
              "email": {
                "type": "string"
              },
              "name": {
                "type": "string"
              }
            }
          }
        }
      ],
      "config": {},
      "position": {
        "x": 600,
        "y": 100
      }
    }
  ]
  JSON

  edges = <<-JSON
  [
    {
      "id": "e1",
      "source_node_id": "input-1",
      "source_port": "customer_id",
      "target_node_id": "get-customer",
      "target_port": "customer_id"
    },
    {
      "id": "e2",
      "source_node_id": "get-customer",
      "source_port": "result",
      "target_node_id": "output-1",
      "target_port": "result"
    }
  ]
  JSON
}
