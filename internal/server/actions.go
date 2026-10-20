package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// actionFilter is shared by the project-wide and per-server action lists.
type actionFilter struct {
	IDs    []int64  `json:"ids,omitempty" jsonschema:"Action ids. Hetzner requires at least one id for the project-wide list. Use list_server_actions for history."`
	Status string   `json:"status,omitempty" jsonschema:"success, running, or error. Omit for every status."`
	Sort   []string `json:"sort,omitempty" jsonschema:"id, id:asc, or id:desc. Default id:desc."`
	page
}

func (f actionFilter) listOpts() (hcloud.ActionListOpts, error) {
	opts := hcloud.ActionListOpts{ListOpts: f.page.listOpts(), ID: f.IDs}
	if f.Status != "" {
		status := hcloud.ActionStatus(f.Status)
		switch status {
		case hcloud.ActionStatusRunning, hcloud.ActionStatusSuccess, hcloud.ActionStatusError:
			opts.Status = []hcloud.ActionStatus{status}
		default:
			return opts, fmt.Errorf("status must be success, running, or error")
		}
	}
	for _, value := range f.Sort {
		key := strings.TrimSpace(value)
		switch key {
		case "id", "id:asc", "id:desc":
			opts.Sort = append(opts.Sort, key)
		default:
			return opts, fmt.Errorf("sort must be id, id:asc, or id:desc")
		}
	}
	return opts, nil
}

type actionsPageOut struct {
	Actions   []actionView `json:"actions"`
	Page      pageInfo     `json:"page"`
	RateLimit rateInfo     `json:"rate_limit"`
}

type listActionsIn struct {
	actionFilter
}

// listActions reads the project-wide action list. Hetzner requires ids here,
// so history for one server belongs to list_server_actions.
func (a *App) listActions(ctx context.Context, _ *mcp.CallToolRequest, in listActionsIn) (*mcp.CallToolResult, actionsPageOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, actionsPageOut{}, err
	}
	if len(in.IDs) == 0 {
		return nil, actionsPageOut{}, fmt.Errorf("Hetzner requires at least one action id for the project-wide list. Pass ids, or use list_server_actions for one server's history")
	}
	opts, err := in.listOpts()
	if err != nil {
		return nil, actionsPageOut{}, err
	}
	items, resp, err := client.Action.List(ctx, opts)
	if err != nil {
		return nil, actionsPageOut{}, err
	}
	out := actionsPageOut{Actions: viewActions(items)}
	out.Page, out.RateLimit = metaOf(resp)
	return nil, out, nil
}

type listServerActionsIn struct {
	ID int64 `json:"id" jsonschema:"Server id. Actions for this server only."`
	actionFilter
}

func (a *App) listServerActions(ctx context.Context, _ *mcp.CallToolRequest, in listServerActionsIn) (*mcp.CallToolResult, actionsPageOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, actionsPageOut{}, err
	}
	if len(in.IDs) > 0 {
		return nil, actionsPageOut{}, fmt.Errorf("ids belongs to list_actions; this tool already scopes to one server")
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, actionsPageOut{}, err
	}
	opts, err := in.listOpts()
	if err != nil {
		return nil, actionsPageOut{}, err
	}
	items, resp, err := client.Server.Action.ListFor(ctx, server, opts)
	if err != nil {
		return nil, actionsPageOut{}, err
	}
	out := actionsPageOut{Actions: viewActions(items)}
	out.Page, out.RateLimit = metaOf(resp)
	return nil, out, nil
}
