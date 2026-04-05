# E2E Comprehensive
#
# Terraform representation of the E2E Comprehensive test graph.
# See graph.json for the equivalent JSON representation.

variable "name" {
  type    = string
  default = "E2E Comprehensive"
}

resource "brockley_graph" "test" {
  name        = var.name
  namespace   = "e2e-tests"
  description = "E2E test: transforms, conditionals, parallel fork/join, skip propagation, exclusive fan-in, broad expression coverage."

  nodes = <<-JSON
  [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [
        {
          "name": "data",
          "schema": {
            "type": "object",
            "properties": {
              "text": {
                "type": "string"
              },
              "number": {
                "type": "number"
              },
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "items": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              },
              "tags": {
                "type": "object",
                "properties": {
                  "color": {
                    "type": "string"
                  },
                  "size": {
                    "type": "string"
                  }
                }
              }
            },
            "required": [
              "text",
              "number",
              "tier",
              "priority",
              "items",
              "tags"
            ]
          }
        }
      ],
      "output_ports": [
        {
          "name": "data",
          "schema": {
            "type": "object",
            "properties": {
              "text": {
                "type": "string"
              },
              "number": {
                "type": "number"
              },
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "items": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              },
              "tags": {
                "type": "object",
                "properties": {
                  "color": {
                    "type": "string"
                  },
                  "size": {
                    "type": "string"
                  }
                }
              }
            },
            "required": [
              "text",
              "number",
              "tier",
              "priority",
              "items",
              "tags"
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
      "id": "transform-a",
      "name": "Transform A",
      "type": "transform",
      "input_ports": [
        {
          "name": "data",
          "schema": {
            "type": "object",
            "properties": {
              "text": {
                "type": "string"
              },
              "number": {
                "type": "number"
              },
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "items": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              },
              "tags": {
                "type": "object",
                "properties": {
                  "color": {
                    "type": "string"
                  },
                  "size": {
                    "type": "string"
                  }
                }
              }
            }
          }
        }
      ],
      "output_ports": [
        {
          "name": "normalized_text",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "doubled",
          "schema": {
            "type": "number"
          }
        },
        {
          "name": "has_items",
          "schema": {
            "type": "boolean"
          }
        },
        {
          "name": "tag_keys",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        {
          "name": "safe_value",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "tier",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "priority",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "message",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "items",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        }
      ],
      "config": {
        "expressions": {
          "normalized_text": "input.data.text | trim | lower",
          "doubled": "input.data.number * 2",
          "has_items": "(input.data.items | length) > 0",
          "tag_keys": "input.data.tags | keys",
          "safe_value": "input.data.missing?.nested ?? \"fallback\"",
          "tier": "input.data.tier",
          "priority": "input.data.priority",
          "message": "input.data.text | trim",
          "items": "input.data.items"
        }
      },
      "position": {
        "x": 200,
        "y": 150
      }
    },
    {
      "id": "transform-b",
      "name": "Transform B (arithmetic)",
      "type": "transform",
      "input_ports": [
        {
          "name": "doubled",
          "schema": {
            "type": "number"
          }
        },
        {
          "name": "tier",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "priority",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "message",
          "schema": {
            "type": "string"
          }
        }
      ],
      "output_ports": [
        {
          "name": "calc",
          "schema": {
            "type": "number"
          }
        },
        {
          "name": "as_string",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "is_large",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "tier",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "priority",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "message",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {
        "expressions": {
          "calc": "(input.doubled + 10) * 3 - input.doubled % 4",
          "as_string": "input.doubled | toString",
          "is_large": "input.doubled > 50 ? \"large\" : \"small\"",
          "tier": "input.tier",
          "priority": "input.priority",
          "message": "input.message"
        }
      },
      "position": {
        "x": 450,
        "y": 0
      }
    },
    {
      "id": "transform-c",
      "name": "Transform C (string ops)",
      "type": "transform",
      "input_ports": [
        {
          "name": "normalized_text",
          "schema": {
            "type": "string"
          }
        }
      ],
      "output_ports": [
        {
          "name": "upper_text",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "words",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        {
          "name": "word_count",
          "schema": {
            "type": "number"
          }
        },
        {
          "name": "replaced",
          "schema": {
            "type": "string"
          }
        }
      ],
      "config": {
        "expressions": {
          "upper_text": "input.normalized_text | upper",
          "words": "input.normalized_text | split(\" \")",
          "word_count": "(input.normalized_text | split(\" \") | length)",
          "replaced": "input.normalized_text | replace(\"o\", \"0\")"
        }
      },
      "position": {
        "x": 450,
        "y": 150
      }
    },
    {
      "id": "transform-d",
      "name": "Transform D (array ops)",
      "type": "transform",
      "input_ports": [
        {
          "name": "has_items",
          "schema": {
            "type": "boolean"
          }
        },
        {
          "name": "tag_keys",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        {
          "name": "items",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        }
      ],
      "output_ports": [
        {
          "name": "has_items_str",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "sorted_keys",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        {
          "name": "first_key",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "unique_items",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        {
          "name": "filtered",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        {
          "name": "reversed",
          "schema": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        {
          "name": "contains_hello",
          "schema": {
            "type": "boolean"
          }
        }
      ],
      "config": {
        "expressions": {
          "has_items_str": "input.has_items | toString",
          "sorted_keys": "input.tag_keys | sort",
          "first_key": "input.tag_keys | sort | first",
          "unique_items": "input.items | unique",
          "filtered": "input.items | filter(x => x != \"skip\")",
          "reversed": "input.items | reverse",
          "contains_hello": "input.items | contains(\"hello\")"
        }
      },
      "position": {
        "x": 450,
        "y": 300
      }
    },
    {
      "id": "joiner",
      "name": "Joiner Transform",
      "type": "transform",
      "input_ports": [
        {
          "name": "tier",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "priority",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "message",
          "schema": {
            "type": "string"
          }
        },
        {
          "name": "calc",
          "schema": {
            "type": "number"
          }
        }
      ],
      "output_ports": [
        {
          "name": "value",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
                "type": "string"
              }
            }
          }
        }
      ],
      "config": {
        "expressions": {
          "value": "{tier: input.tier, priority: input.priority, message: input.message, summary: input.calc | toString}"
        }
      },
      "position": {
        "x": 700,
        "y": 0
      }
    },
    {
      "id": "conditional-a",
      "name": "Conditional A",
      "type": "conditional",
      "input_ports": [
        {
          "name": "value",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
                "type": "string"
              }
            }
          }
        }
      ],
      "output_ports": [
        {
          "name": "premium",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
                "type": "string"
              }
            }
          }
        },
        {
          "name": "standard",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
                "type": "string"
              }
            }
          }
        }
      ],
      "config": {
        "branches": [
          {
            "label": "premium",
            "condition": "input.value.tier == \"premium\""
          }
        ],
        "default_label": "standard"
      },
      "position": {
        "x": 900,
        "y": 0
      }
    },
    {
      "id": "conditional-b",
      "name": "Conditional B (nested)",
      "type": "conditional",
      "input_ports": [
        {
          "name": "value",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
                "type": "string"
              }
            }
          }
        }
      ],
      "output_ports": [
        {
          "name": "high",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
                "type": "string"
              }
            }
          }
        },
        {
          "name": "low",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
                "type": "string"
              }
            }
          }
        }
      ],
      "config": {
        "branches": [
          {
            "label": "high",
            "condition": "input.value.priority == \"high\""
          }
        ],
        "default_label": "low"
      },
      "position": {
        "x": 1100,
        "y": -100
      }
    },
    {
      "id": "handler-high",
      "name": "Handler High",
      "type": "transform",
      "input_ports": [
        {
          "name": "value",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
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
            "type": "string"
          }
        }
      ],
      "config": {
        "expressions": {
          "result": "\"PREMIUM-HIGH: \" + (input.value.message | upper)"
        }
      },
      "position": {
        "x": 1300,
        "y": -150
      }
    },
    {
      "id": "handler-low",
      "name": "Handler Low",
      "type": "transform",
      "input_ports": [
        {
          "name": "value",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
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
            "type": "string"
          }
        }
      ],
      "config": {
        "expressions": {
          "result": "\"premium-low: \" + (input.value.message | lower)"
        }
      },
      "position": {
        "x": 1300,
        "y": -50
      }
    },
    {
      "id": "handler-standard",
      "name": "Handler Standard",
      "type": "transform",
      "input_ports": [
        {
          "name": "value",
          "schema": {
            "type": "object",
            "properties": {
              "tier": {
                "type": "string"
              },
              "priority": {
                "type": "string"
              },
              "message": {
                "type": "string"
              },
              "summary": {
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
            "type": "string"
          }
        }
      ],
      "config": {
        "expressions": {
          "result": "\"standard: \" + input.value.message"
        }
      },
      "position": {
        "x": 1100,
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
      "config": {},
      "position": {
        "x": 1500,
        "y": 0
      }
    }
  ]
  JSON

  edges = <<-JSON
  [
    {
      "id": "e1",
      "source_node_id": "input-1",
      "source_port": "data",
      "target_node_id": "transform-a",
      "target_port": "data"
    },
    {
      "id": "e2",
      "source_node_id": "transform-a",
      "source_port": "doubled",
      "target_node_id": "transform-b",
      "target_port": "doubled"
    },
    {
      "id": "e3",
      "source_node_id": "transform-a",
      "source_port": "tier",
      "target_node_id": "transform-b",
      "target_port": "tier"
    },
    {
      "id": "e4",
      "source_node_id": "transform-a",
      "source_port": "priority",
      "target_node_id": "transform-b",
      "target_port": "priority"
    },
    {
      "id": "e5",
      "source_node_id": "transform-a",
      "source_port": "message",
      "target_node_id": "transform-b",
      "target_port": "message"
    },
    {
      "id": "e6",
      "source_node_id": "transform-a",
      "source_port": "normalized_text",
      "target_node_id": "transform-c",
      "target_port": "normalized_text"
    },
    {
      "id": "e7",
      "source_node_id": "transform-a",
      "source_port": "has_items",
      "target_node_id": "transform-d",
      "target_port": "has_items"
    },
    {
      "id": "e8",
      "source_node_id": "transform-a",
      "source_port": "tag_keys",
      "target_node_id": "transform-d",
      "target_port": "tag_keys"
    },
    {
      "id": "e9",
      "source_node_id": "transform-a",
      "source_port": "items",
      "target_node_id": "transform-d",
      "target_port": "items"
    },
    {
      "id": "e10",
      "source_node_id": "transform-b",
      "source_port": "calc",
      "target_node_id": "joiner",
      "target_port": "calc"
    },
    {
      "id": "e11",
      "source_node_id": "transform-b",
      "source_port": "tier",
      "target_node_id": "joiner",
      "target_port": "tier"
    },
    {
      "id": "e12",
      "source_node_id": "transform-b",
      "source_port": "priority",
      "target_node_id": "joiner",
      "target_port": "priority"
    },
    {
      "id": "e13",
      "source_node_id": "transform-b",
      "source_port": "message",
      "target_node_id": "joiner",
      "target_port": "message"
    },
    {
      "id": "e14",
      "source_node_id": "joiner",
      "source_port": "value",
      "target_node_id": "conditional-a",
      "target_port": "value"
    },
    {
      "id": "e17",
      "source_node_id": "conditional-a",
      "source_port": "premium",
      "target_node_id": "conditional-b",
      "target_port": "value"
    },
    {
      "id": "e18",
      "source_node_id": "conditional-a",
      "source_port": "standard",
      "target_node_id": "handler-standard",
      "target_port": "value"
    },
    {
      "id": "e19",
      "source_node_id": "conditional-b",
      "source_port": "high",
      "target_node_id": "handler-high",
      "target_port": "value"
    },
    {
      "id": "e20",
      "source_node_id": "conditional-b",
      "source_port": "low",
      "target_node_id": "handler-low",
      "target_port": "value"
    },
    {
      "id": "e21",
      "source_node_id": "handler-high",
      "source_port": "result",
      "target_node_id": "output-1",
      "target_port": "result"
    },
    {
      "id": "e22",
      "source_node_id": "handler-low",
      "source_port": "result",
      "target_node_id": "output-1",
      "target_port": "result"
    },
    {
      "id": "e23",
      "source_node_id": "handler-standard",
      "source_port": "result",
      "target_node_id": "output-1",
      "target_port": "result"
    }
  ]
  JSON
}
