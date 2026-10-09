# Hetzner Cloud MCP

An MCP server for managing Hetzner Cloud projects. The server holds the project token and performs API requests on behalf of the MCP client.

It uses [`hcloud-go`](https://github.com/hetznercloud/hcloud-go) for everyday operations. Everything else is supported via direct OpenAPI `cloud_api` fallback.

## Authentication

The server resolves a token at startup via environment variable or the `hcloud` CLI:
1. `HCLOUD_TOKEN`
2. Context named by `HCLOUD_CONTEXT` in `~/.config/hcloud/cli.toml`
3. The file's `active_context`

## Usage

```bash
# stdio transport (default)
./mcp-hetzner

# streamable HTTP
./mcp-hetzner --transport http --host 127.0.0.1 --port 8080
```

External listeners require an MCP bearer token:
```bash
MCP_HOST=0.0.0.0 MCP_AUTH_TOKEN=secret ./mcp-hetzner --transport http
```

### Tools

- **Core**: `describe_project`, `list_servers`, `get_server`, `create_server`, `update_server`, `set_server_protection`, `delete_server`, `server_power`
- **Catalog**: `list_server_types`, `list_images`, `list_locations`
- **SSH Keys**: `list_ssh_keys`, `create_ssh_key`, `delete_ssh_key`
- **Volumes**: `list_volumes`, `create_volume`, `volume_action`
- **Firewalls**: `list_firewalls`, `get_firewall`, `create_firewall`, `set_firewall_rules`, `apply_firewall`, `delete_firewall`
- **Inventory**: `list_networks`, `list_load_balancers`, `list_primary_ips`, `list_floating_ips`
- **Raw API**: `find_operations`, `cloud_get`, `cloud_api`

## Build and Test

```bash
go build -o mcp-hetzner ./cmd/mcp-hetzner
go test ./...
```

## License
MIT
