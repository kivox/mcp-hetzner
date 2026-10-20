package server

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type rescueIn struct {
	ID      int64    `json:"id" jsonschema:"Server id."`
	Type    string   `json:"type,omitempty" jsonschema:"linux64 or linux32. Default linux64."`
	SSHKeys []string `json:"ssh_keys,omitempty" jsonschema:"SSH key names or ids to install in rescue mode. Omit to get a root password instead."`
	Wait    *bool    `json:"wait,omitempty" jsonschema:"Wait until the action finishes. Default true."`
}

type rescueOut struct {
	Server       serverView  `json:"server"`
	Action       *actionView `json:"action,omitempty"`
	RootPassword string      `json:"root_password,omitempty" jsonschema:"Set only when no SSH key was used. Show it once."`
}

// enableServerRescue turns rescue mode on. Hetzner only accepts this while the
// server is off, so the check happens here with an actionable message instead
// of an API error the agent has to decode.
func (a *App) enableServerRescue(ctx context.Context, _ *mcp.CallToolRequest, in rescueIn) (*mcp.CallToolResult, rescueOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, rescueOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, rescueOut{}, err
	}
	if server.Status != hcloud.ServerStatusOff {
		return nil, rescueOut{}, fmt.Errorf("server %q is %s; rescue mode needs it off. Call server_power action shutdown first", server.Name, server.Status)
	}
	kind := in.Type
	if kind == "" {
		kind = string(hcloud.ServerRescueTypeLinux64)
	}
	switch hcloud.ServerRescueType(kind) {
	case hcloud.ServerRescueTypeLinux64, hcloud.ServerRescueTypeLinux32:
	default:
		return nil, rescueOut{}, fmt.Errorf("type must be linux64 or linux32")
	}
	var keys []*hcloud.SSHKey
	for _, ref := range in.SSHKeys {
		key, _, keyErr := client.SSHKey.Get(ctx, ref)
		if keyErr != nil || key == nil {
			return nil, rescueOut{}, fmt.Errorf("ssh key %q: %w", ref, errOrMissing(keyErr))
		}
		keys = append(keys, key)
	}
	result, _, err := client.Server.EnableRescue(ctx, server, hcloud.ServerEnableRescueOpts{
		Type:    hcloud.ServerRescueType(kind),
		SSHKeys: keys,
	})
	if err != nil {
		return nil, rescueOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, result.Action); err != nil {
			return nil, rescueOut{}, err
		}
	}
	if fresh, _, getErr := client.Server.GetByID(ctx, server.ID); getErr == nil && fresh != nil {
		server = fresh
	}
	return nil, rescueOut{
		Server:       viewServer(server),
		Action:       viewAction(result.Action),
		RootPassword: result.RootPassword,
	}, nil
}

type disableRescueIn struct {
	ID   int64 `json:"id" jsonschema:"Server id."`
	Wait *bool `json:"wait,omitempty" jsonschema:"Wait until the action finishes. Default true."`
}

func (a *App) disableServerRescue(ctx context.Context, _ *mcp.CallToolRequest, in disableRescueIn) (*mcp.CallToolResult, rescueOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, rescueOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, rescueOut{}, err
	}
	action, _, err := client.Server.DisableRescue(ctx, server)
	if err != nil {
		return nil, rescueOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, action); err != nil {
			return nil, rescueOut{}, err
		}
	}
	if fresh, _, getErr := client.Server.GetByID(ctx, server.ID); getErr == nil && fresh != nil {
		server = fresh
	}
	return nil, rescueOut{Server: viewServer(server), Action: viewAction(action)}, nil
}
