# Hetzner Cloud MCP

An MCP server for agents that manage one Hetzner Cloud project. The server holds the project token and performs every HTTP request. A tool call never accepts a token, an `Authorization` header, or a hostname.

It uses [`hcloud-go`](https://github.com/hetznercloud/hcloud-go) for the Cloud API and a compact index of the [official OpenAPI spec](https://docs.hetzner.cloud/cloud.spec.json). The everyday resources are named tools. Everything else is `find_operations`, then `cloud_api`.

## Authentication

The server resolves a token at startup. An environment variable wins. Otherwise it reads the active context from the [hcloud CLI](https://github.com/hetznercloud/cli) config.

1. `HCLOUD_TOKEN`
2. Context named by `HCLOUD_CONTEXT` in `~/.config/hcloud/cli.toml`
3. The file's `active_context`

`HCLOUD_CONFIG` overrides the config path. The token is not written to logs. If neither source is set, the process still starts and tool calls return a clear error so the agent can tell you.

## Run

```bash
go build -o bin/mcp-hetzner ./cmd/mcp-hetzner
./bin/mcp-hetzner                  # stdio, for an agent on this machine
./bin/mcp-hetzner --transport http # streamable HTTP on 127.0.0.1:8080
```

Claude, Cursor, or another MCP client should launch the stdio binary. Do not put the token in the client config if the CLI context is already set.

```json
{
  "mcpServers": {
    "hetzner": {
      "command": "/path/to/mcp-hetzner"
    }
  }
}
```

## How an agent should use it

Read before writing. `list_*` and `get_*` return one page. When `next_page` is set, call again with that page.

`delete_server`, `delete_ssh_key`, `delete_firewall`, and `volume_action` delete require `confirm_name` to match the live name.

`server_power` prefers `shutdown`. `poweroff` and `reset` cut power.

`create_server` waits until Hetzner finishes unless `wait` is false. A root password comes back only when no SSH key was used.

`find_operations` searches the spec. `cloud_api` then sends that method and path. The path must be one the spec allows.

## Tools

| Tool | What the server does |
| --- | --- |
| `list_servers`, `get_server`, `create_server`, `delete_server`, `server_power` | Server inventory and power |
| `list_server_types`, `list_images`, `list_locations` | Choices for creating a server |
| `list_ssh_keys`, `create_ssh_key`, `delete_ssh_key` | Project SSH keys |
| `list_volumes`, `create_volume`, `volume_action` | Volumes |
| `list_firewalls`, `get_firewall`, `create_firewall`, `set_firewall_rules`, `apply_firewall`, `delete_firewall` | Firewalls |
| `get_action` | Poll an action |
| `list_networks`, `list_load_balancers`, `list_primary_ips`, `list_floating_ips` | Project inventory. Changes go through `cloud_api` |
| `find_operations`, `cloud_get`, `cloud_api` | The rest of the Cloud API. GET is read-only. Mutations stay on `cloud_api` |

List results set `incomplete` when another page exists and `rate_limit.slow_down` when the project is close to the Hetzner limit. Server results include delete protection, public IPv6, and private IPs. A protected resource is refused before the API call. Each HTTP request times out after 30 seconds. Action polling uses a fresh request per tick and waits up to three minutes. Responses larger than 256KB are rejected with a hint to paginate.

## Develop

```bash
go test ./...
```

`internal/catalog/operations.json` is the operation index from `https://docs.hetzner.cloud/cloud.spec.json`. Regenerate it when Hetzner publishes a new spec. Do not commit the 3 MB source document.

## License

MIT
