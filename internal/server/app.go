package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `You are talking to the Hetzner Cloud MCP server for one project.
The server authenticates and performs every HTTP request. Never pass an API token, an Authorization header, or a host name.

Read before you change anything. Use list or get tools, then act on the id and name they return.
delete tools require confirm_name to equal the live resource name.
Prefer server_power action "shutdown" (graceful) over "poweroff" (hard).
create_server and power actions wait until Hetzner finishes unless you set wait to false. A root password is returned only when the server was created without an SSH key. Repeat it to the user once.
When no dedicated tool exists, call find_operations. Then cloud_get for a GET, or cloud_api for POST, PUT, or DELETE. Copy the path. A /v1 prefix is accepted. The server sends the request.
If incomplete or next_page is set, call again with that page. The first page is not the whole project.
If rate_limit.slow_down is true, stop and wait instead of retrying.
If protection.delete is true, do not call a delete tool. If protection.rebuild is true, do not call rebuild_server. Disable protection first.`

// App is the authenticated Hetzner client shared by every tool.
type App struct {
	Client *hcloud.Client
}

// NewServer builds the MCP server. Client may be nil; tool calls then explain
// that the server has no project token. Listing tools still works.
func NewServer(app *App) *mcp.Server {
	if app == nil {
		app = &App{}
	}
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-hetzner",
		Version: Version,
		Title:   "Hetzner Cloud",
	}, &mcp.ServerOptions{Instructions: instructions})
	app.register(s)
	return s
}

func (a *App) api() (*hcloud.Client, error) {
	if a == nil || a.Client == nil {
		return nil, errors.New("this MCP server has no Hetzner token. Set HCLOUD_TOKEN or run hcloud context create. Do not pass a token into a tool")
	}
	return a.Client, nil
}

func (a *App) wait(ctx context.Context, actions ...*hcloud.Action) error {
	live := actions[:0]
	for _, action := range actions {
		if action != nil {
			live = append(live, action)
		}
	}
	if len(live) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := a.Client.Action.WaitFor(ctx, live...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("Hetzner is still running %d action(s) after 3m. Read them with get_action instead of repeating the change: %w", len(live), err)
		}
		return fmt.Errorf("action did not succeed: %w", err)
	}
	return nil
}

func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true}
}

// additive marks a tool that changes state without destroying anything and
// reaches the same result when it is called twice.
func additive(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: boolPtr(false), IdempotentHint: true}
}

// creates marks a tool that allocates a new resource. Calling it twice makes
// two resources, so it must not be advertised as idempotent.
func creates(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: boolPtr(false), IdempotentHint: false}
}

func destructive(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: boolPtr(true)}
}

func boolPtr(v bool) *bool { return &v }
