# Quickstart

Get Brockley running locally and execute your first graph in under 5 minutes.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [Git](https://git-scm.com/)
- `curl` (for testing the API)

## 1. Clone the Repository

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai
```

## 2. Start the Development Environment

```bash
make dev
```

This starts the following services via Docker Compose:

| Service | Port | Purpose |
|---------|------|---------|
| **server** | `localhost:8000` | API server |
| **worker** | (internal) | Async task processor |
| **coderunner** | (internal) | Code execution runtime (Python) |
| **web-ui** | `localhost:3000` | Visual graph editor |
| **postgresql** | `localhost:5432` | Database |
| **redis** | `localhost:6379` | Task queue and event streaming |

A seed container also loads example graphs automatically.

Wait until you see log output from the server indicating it's ready:

```
server-1  | level=INFO msg="server listening" addr=0.0.0.0:8000
```

## 3. Verify the Server is Running

```bash
curl http://localhost:8000/health
```

Expected response:

```json
{"status": "ok"}
```

Check readiness (confirms database and Redis are connected):

```bash
curl http://localhost:8000/health/ready
```

```json
{"status": "ready", "checks": {"database": "ok", "redis": "ok"}}
```

## 4. Open the Web UI

Open [http://localhost:3000](http://localhost:3000) in your browser. You should see the Brockley visual editor with any seeded example graphs.

## 5. Create a Graph via the API

Create a simple graph that takes a `message` input and passes it through to an output:

```bash
curl -s -X POST http://localhost:8000/api/v1/graphs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hello-world",
    "description": "A simple passthrough graph",
    "namespace": "default",
    "nodes": [
      {
        "id": "input-1",
        "name": "Input",
        "type": "input",
        "input_ports": [],
        "output_ports": [
          {
            "name": "message",
            "schema": {"type": "string"}
          }
        ],
        "config": {}
      },
      {
        "id": "output-1",
        "name": "Output",
        "type": "output",
        "input_ports": [
          {
            "name": "result",
            "schema": {"type": "string"}
          }
        ],
        "output_ports": [],
        "config": {}
      }
    ],
    "edges": [
      {
        "id": "edge-1",
        "source_node_id": "input-1",
        "source_port": "message",
        "target_node_id": "output-1",
        "target_port": "result"
      }
    ]
  }' | jq .
```

The response includes the created graph with an `id`. Save this ID for the next step.

## 6. Execute the Graph

Replace `GRAPH_ID` with the ID from the previous response:

```bash
curl -s -X POST http://localhost:8000/api/v1/executions \
  -H "Content-Type: application/json" \
  -d '{
    "graph_id": "GRAPH_ID",
    "input": {
      "message": "Hello, Brockley!"
    },
    "mode": "sync"
  }' | jq .
```

Expected response:

```json
{
  "id": "exec-...",
  "status": "completed",
  "output": {
    "result": "Hello, Brockley!"
  }
}
```

## 7. List Your Graphs

```bash
curl -s http://localhost:8000/api/v1/graphs | jq .
```

## 8. Stop the Environment

```bash
make dev-down
```

## Alternative: Using the CLI

If you prefer the command line over `curl`, install the Brockley CLI:

```bash
go install github.com/brockleyai/brockleyai/cmd/brockley@latest
```

Then deploy and execute graphs directly:

```bash
# Validate a graph locally (no server needed)
brockley validate -f examples/comprehensive/graph.json

# Deploy a graph
brockley deploy -f examples/comprehensive/graph.json

# Execute a graph (use the ID returned by deploy)
brockley invoke GRAPH_ID --input '{"data": {"text": "hello", "number": 42, "tier": "premium", "priority": "high", "items": ["a","b"], "tags": {"color": "blue", "size": "large"}}}' --sync
```

See the [CLI reference](../cli/) for all available commands.

## Alternative: Build with a Coding Agent

If you use Claude Code, Cursor, Copilot, or another coding agent, you can skip writing JSON by hand entirely. Brockley ships a [coding agent skill](../../coding-agent-skills/SKILL.md) that lets your agent produce valid graph definitions from natural language descriptions.

Set up the skill for your agent (one-time):

```bash
# Claude Code -- add the skill as a command
cp coding-agent-skills/SKILL.md .claude/commands/brockley.md

# Then ask your agent:
# "Build me a graph that takes a message input and passes it through to an output"
```

Your agent generates valid, deployable graph JSON. Deploy it with the CLI or API:

```bash
brockley deploy -f graph.json
```

See the [coding agent skills README](../../coding-agent-skills/README.md) for setup instructions for Cursor, Copilot, and other agents.

## What's Next

- [Build Your First Graph](first-graph.md) -- a more interesting tutorial with a transform node
- [Architecture Overview](architecture-overview.md) -- understand the components
- [Core Concepts: Graphs](../concepts/graphs.md) -- deep dive into graph structure
- [Build with Coding Agents](../../coding-agent-skills/README.md) -- full setup guide for coding agent graph authoring
