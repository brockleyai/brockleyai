# E2E LLM Pipeline
#
# Terraform representation of the E2E LLM Pipeline test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E LLM Pipeline"
}

variable "openrouter_api_key" {
  type      = string
  sensitive = true
  default   = ""
}

resource "brockley_graph" "test" {
  name        = var.name
  namespace   = "e2e-tests"
  description = "E2E test: LLM classification (JSON response), conditional routing on structured output, LLM text response, template rendering with #if/#each, env var expansion."

  nodes = <<-JSON
  [
    {
      "id": "input-1",
      "name": "Ticket Input",
      "type": "input",
      "input_ports": [
        {
          "name": "ticket",
          "schema": {
            "type": "object",
            "properties": {
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              },
              "history": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              }
            },
            "required": [
              "subject",
              "body"
            ]
          }
        }
      ],
      "output_ports": [
        {
          "name": "ticket",
          "schema": {
            "type": "object",
            "properties": {
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              },
              "history": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              }
            },
            "required": [
              "subject",
              "body"
            ]
          }
        }
      ],
      "config": {},
      "position": {
        "x": 0,
        "y": 150
      }
    },
    {
      "id": "classify",
      "name": "Classify Ticket",
      "type": "llm",
      "input_ports": [
        {
          "name": "ticket",
          "schema": {
            "type": "object",
            "properties": {
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              },
              "history": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              }
            },
            "required": [
              "subject",
              "body"
            ]
          }
        }
      ],
      "output_ports": [
        {
          "name": "response",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              }
            },
            "required": [
              "category"
            ]
          }
        }
      ],
      "config": {
        "provider": "openrouter",
        "model": "openai/gpt-oss-20b",
        "api_key": "${var.openrouter_api_key}",
        "system_prompt": "You are a support ticket classifier. You must respond with valid JSON only. Classify the ticket into exactly one category: billing, technical, or general.",
        "user_prompt": "Classify this ticket.\nSubject: {{input.ticket.subject}}\nBody: {{input.ticket.body}}\n{{#if input.ticket.history}}\nPrevious messages:\n{{#each input.ticket.history}}\n- {{this}}\n{{/each}}\n{{/if}}\nRespond with JSON: {\"category\": \"billing\"|\"technical\"|\"general\"}",
        "variables": [
          {
            "name": "ticket",
            "schema": {
              "type": "object",
              "properties": {
                "subject": {
                  "type": "string"
                },
                "body": {
                  "type": "string"
                },
                "history": {
                  "type": "array",
                  "items": {
                    "type": "string"
                  }
                }
              },
              "required": [
                "subject",
                "body"
              ]
            }
          }
        ],
        "response_format": "json",
        "output_schema": {
          "type": "object",
          "properties": {
            "category": {
              "type": "string",
              "enum": [
                "billing",
                "technical",
                "general"
              ]
            }
          },
          "required": [
            "category"
          ]
        },
        "temperature": 0.0,
        "max_tokens": 128
      },
      "position": {
        "x": 300,
        "y": 150
      }
    },
    {
      "id": "merge",
      "name": "Merge Ticket + Classification",
      "type": "transform",
      "input_ports": [
        {
          "name": "ticket",
          "schema": {
            "type": "object",
            "properties": {
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              },
              "history": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              }
            },
            "required": [
              "subject",
              "body"
            ]
          }
        },
        {
          "name": "classification",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              }
            },
            "required": [
              "category"
            ]
          }
        }
      ],
      "output_ports": [
        {
          "name": "merged",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              },
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              }
            },
            "required": [
              "category",
              "subject",
              "body"
            ]
          }
        }
      ],
      "config": {
        "expressions": {
          "merged": "{category: input.classification.category, subject: input.ticket.subject, body: input.ticket.body}"
        }
      },
      "position": {
        "x": 450,
        "y": 150
      }
    },
    {
      "id": "router",
      "name": "Route by Category",
      "type": "conditional",
      "input_ports": [
        {
          "name": "value",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              },
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              }
            },
            "required": [
              "category"
            ]
          }
        }
      ],
      "output_ports": [
        {
          "name": "billing",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              },
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              }
            },
            "required": [
              "category"
            ]
          }
        },
        {
          "name": "technical",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              },
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              }
            },
            "required": [
              "category"
            ]
          }
        },
        {
          "name": "general",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              },
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              }
            },
            "required": [
              "category"
            ]
          }
        }
      ],
      "config": {
        "branches": [
          {
            "label": "billing",
            "condition": "input.value.category == \"billing\""
          },
          {
            "label": "technical",
            "condition": "input.value.category == \"technical\""
          }
        ],
        "default_label": "general"
      },
      "position": {
        "x": 600,
        "y": 150
      }
    },
    {
      "id": "billing-responder",
      "name": "Billing Responder",
      "type": "llm",
      "input_ports": [
        {
          "name": "data",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              },
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              }
            },
            "required": [
              "subject",
              "body"
            ]
          }
        }
      ],
      "output_ports": [
        {
          "name": "response_text",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {
        "provider": "openrouter",
        "model": "openai/gpt-oss-20b",
        "api_key": "${var.openrouter_api_key}",
        "system_prompt": "You are a helpful billing support agent. Be empathetic and provide clear next steps. Keep responses under 3 sentences.",
        "user_prompt": "Customer ticket:\nSubject: {{input.data.subject}}\n{{input.data.body}}\n\nDraft a helpful reply:",
        "variables": [
          {
            "name": "data",
            "schema": {
              "type": "object",
              "properties": {
                "subject": {
                  "type": "string"
                },
                "body": {
                  "type": "string"
                }
              },
              "required": [
                "subject",
                "body"
              ]
            }
          }
        ],
        "response_format": "text",
        "max_tokens": 256
      },
      "position": {
        "x": 900,
        "y": 0
      }
    },
    {
      "id": "technical-responder",
      "name": "Technical Responder",
      "type": "llm",
      "input_ports": [
        {
          "name": "data",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              },
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              }
            },
            "required": [
              "subject",
              "body"
            ]
          }
        }
      ],
      "output_ports": [
        {
          "name": "response_text",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {
        "provider": "openrouter",
        "model": "openai/gpt-oss-20b",
        "api_key": "${var.openrouter_api_key}",
        "system_prompt": "You are a helpful technical support engineer. Provide clear troubleshooting steps. Keep responses under 3 sentences.",
        "user_prompt": "Customer ticket:\nSubject: {{input.data.subject}}\n{{input.data.body}}\n\nDraft a helpful reply with troubleshooting steps:",
        "variables": [
          {
            "name": "data",
            "schema": {
              "type": "object",
              "properties": {
                "subject": {
                  "type": "string"
                },
                "body": {
                  "type": "string"
                }
              },
              "required": [
                "subject",
                "body"
              ]
            }
          }
        ],
        "response_format": "text",
        "max_tokens": 256
      },
      "position": {
        "x": 900,
        "y": 150
      }
    },
    {
      "id": "general-responder",
      "name": "General Responder",
      "type": "llm",
      "input_ports": [
        {
          "name": "data",
          "schema": {
            "type": "object",
            "properties": {
              "category": {
                "type": "string"
              },
              "subject": {
                "type": "string"
              },
              "body": {
                "type": "string"
              }
            },
            "required": [
              "subject",
              "body"
            ]
          }
        }
      ],
      "output_ports": [
        {
          "name": "response_text",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {
        "provider": "openrouter",
        "model": "openai/gpt-oss-20b",
        "api_key": "${var.openrouter_api_key}",
        "system_prompt": "You are a friendly customer support agent. Keep responses under 3 sentences.",
        "user_prompt": "Customer ticket:\nSubject: {{input.data.subject}}\n{{input.data.body}}\n\nDraft a helpful reply:",
        "variables": [
          {
            "name": "data",
            "schema": {
              "type": "object",
              "properties": {
                "subject": {
                  "type": "string"
                },
                "body": {
                  "type": "string"
                }
              },
              "required": [
                "subject",
                "body"
              ]
            }
          }
        ],
        "response_format": "text",
        "max_tokens": 256
      },
      "position": {
        "x": 900,
        "y": 300
      }
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [
        {
          "name": "reply",
          "schema": {
            "type": "string"
          }
        }
      ],
      "output_ports": [
        {
          "name": "reply",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {},
      "position": {
        "x": 1200,
        "y": 150
      }
    }
  ]
  JSON

  edges = <<-JSON
  [
    {
      "id": "e1",
      "source_node_id": "input-1",
      "source_port": "ticket",
      "target_node_id": "classify",
      "target_port": "ticket"
    },
    {
      "id": "e2",
      "source_node_id": "classify",
      "source_port": "response",
      "target_node_id": "merge",
      "target_port": "classification"
    },
    {
      "id": "e3",
      "source_node_id": "input-1",
      "source_port": "ticket",
      "target_node_id": "merge",
      "target_port": "ticket"
    },
    {
      "id": "e4",
      "source_node_id": "merge",
      "source_port": "merged",
      "target_node_id": "router",
      "target_port": "value"
    },
    {
      "id": "e5",
      "source_node_id": "router",
      "source_port": "billing",
      "target_node_id": "billing-responder",
      "target_port": "data"
    },
    {
      "id": "e6",
      "source_node_id": "router",
      "source_port": "technical",
      "target_node_id": "technical-responder",
      "target_port": "data"
    },
    {
      "id": "e7",
      "source_node_id": "router",
      "source_port": "general",
      "target_node_id": "general-responder",
      "target_port": "data"
    },
    {
      "id": "e8",
      "source_node_id": "billing-responder",
      "source_port": "response_text",
      "target_node_id": "output-1",
      "target_port": "reply"
    },
    {
      "id": "e9",
      "source_node_id": "technical-responder",
      "source_port": "response_text",
      "target_node_id": "output-1",
      "target_port": "reply"
    },
    {
      "id": "e10",
      "source_node_id": "general-responder",
      "source_port": "response_text",
      "target_node_id": "output-1",
      "target_port": "reply"
    }
  ]
  JSON
}
