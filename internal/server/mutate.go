package server

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type updateServerIn struct {
	ID   int64  `json:"id" jsonschema:"Server id."`
	Name string `json:"name,omitempty" jsonschema:"New hostname. Omit to keep the current name."`
	labelPatch
}

func (a *App) updateServer(ctx context.Context, _ *mcp.CallToolRequest, in updateServerIn) (*mcp.CallToolResult, oneServerOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, oneServerOut{}, err
	}
	if in.Name == "" && in.labelPatch.empty() {
		return nil, oneServerOut{}, fmt.Errorf("pass a name, a label change, or both")
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, oneServerOut{}, err
	}
	labels, changeLabels, err := in.labelPatch.apply(server.Labels)
	if err != nil {
		return nil, oneServerOut{}, err
	}
	opts := hcloud.ServerUpdateOpts{Name: in.Name}
	if changeLabels {
		opts.Labels = labels
	}
	updated, _, err := client.Server.Update(ctx, server, opts)
	if err != nil {
		return nil, oneServerOut{}, err
	}
	return nil, oneServerOut{Server: viewServer(updated)}, nil
}

type protectionIn struct {
	ID      int64 `json:"id"`
	Delete  *bool `json:"delete,omitempty" jsonschema:"When set, turns delete protection on or off."`
	Rebuild *bool `json:"rebuild,omitempty" jsonschema:"When set, turns rebuild protection on or off."`
	Wait    *bool `json:"wait,omitempty" jsonschema:"Wait until the action finishes. Default true."`
}

func (a *App) setServerProtection(ctx context.Context, _ *mcp.CallToolRequest, in protectionIn) (*mcp.CallToolResult, serverPowerOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, serverPowerOut{}, err
	}
	if in.Delete == nil && in.Rebuild == nil {
		return nil, serverPowerOut{}, fmt.Errorf("pass delete, rebuild, or both")
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, serverPowerOut{}, err
	}
	action, _, err := client.Server.ChangeProtection(ctx, server, hcloud.ServerChangeProtectionOpts{
		Delete:  in.Delete,
		Rebuild: in.Rebuild,
	})
	if err != nil {
		return nil, serverPowerOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, action); err != nil {
			return nil, serverPowerOut{}, err
		}
	}
	if fresh, _, getErr := client.Server.GetByID(ctx, server.ID); getErr == nil && fresh != nil {
		server = fresh
	}
	return nil, serverPowerOut{Server: viewServer(server), Action: actionValue(action)}, nil
}
