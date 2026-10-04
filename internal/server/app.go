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
When no dedicated tool exists, call find_operations, then cloud_api with a path copied from that result. The server sends the request.
If next_page is set, call the same tool again with that page. The first page is not the whole project.
If rate_limit.remaining is low, stop and wait instead of retrying immediately.`

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
		Version: "0.2.0",
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
		return fmt.Errorf("action did not succeed: %w", err)
	}
	return nil
}

func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true}
}

func additive(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: boolPtr(false), IdempotentHint: true}
}

func destructive(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: boolPtr(true)}
}

func boolPtr(v bool) *bool { return &v }
