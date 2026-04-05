# brockley auth

Manage CLI authentication configuration.

## Subcommands

### auth set

Save server URL and API key to the config file.

```bash
brockley auth set --server http://localhost:8000 --key your-api-key
```

| Flag | Description |
|------|-------------|
| `--server` | Server URL |
| `--key` | API key |

Config is saved to `~/.brockley/config.json` with restricted permissions (0600).

### auth show

Display the current authentication configuration and its source.

```bash
brockley auth show
```

### auth test

Test connectivity to the server.

```bash
brockley auth test
```

## Credential Resolution Order

1. CLI flags (`--server`, `--api-key`)
2. Environment variables (`BROCKLEY_SERVER_URL`, `BROCKLEY_API_KEY`)
3. Config file (`~/.brockley/config.json`)
