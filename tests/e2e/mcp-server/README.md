# Test MCP Server

A minimal MCP server for E2E testing. Implements JSON-RPC 2.0 over HTTP with three deterministic tools.

## Tools

| Tool | Input | Output | Error case |
|------|-------|--------|------------|
| `echo` | `{message: string}` | The message string (prefixed with `[header]` if `X-Test-Header` is set) | -- |
| `word_count` | `{text: string}` | Word count as string (e.g. `"3"`) | -- |
| `lookup` | `{key: string}` | Ordinal value (`alpha`→`first`, `beta`→`second`, `gamma`→`third`, `delta`→`fourth`) | `isError: true` for unknown keys |

## Endpoints

- `POST /` -- JSON-RPC 2.0 dispatcher (methods: `tools/list`, `tools/call`)
- `GET /health` -- Returns `200 OK`

## Running Locally

```bash
go run .
# Listening on :9090

# Test health
curl http://localhost:9090/health

# List tools
curl -X POST http://localhost:9090 -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Call echo
curl -X POST http://localhost:9090 -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hello"}}}'
```

## Docker

```bash
docker build -t mcp-test-server .
docker run -p 9090:9090 mcp-test-server
```
