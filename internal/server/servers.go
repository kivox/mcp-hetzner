package server

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listServersIn struct {
	page
	Status string `json:"status,omitempty" jsonschema:"Filter by status: running, initializing, starting, stopping, off, deleting, migrating, rebuilding, or unknown."`
}

type listServersOut struct {
	Servers   []serverView `json:"servers"`
	Page      pageInfo     `json:"page"`
	RateLimit rateInfo     `json:"rate_limit"`
}

func (a *App) listServers(ctx context.Context, _ *mcp.CallToolRequest, in listServersIn) (*mcp.CallToolResult, listServersOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, listServersOut{}, err
	}
	opts := hcloud.ServerListOpts{ListOpts: in.listOpts()}
	if in.Status != "" {
		opts.Status = []hcloud.ServerStatus{hcloud.ServerStatus(in.Status)}
	}
	servers, resp, err := client.Server.List(ctx, opts)
	if err != nil {
		return nil, listServersOut{}, err
	}
	out := listServersOut{}
	out.Page, out.RateLimit = metaOf(resp)
	for _, server := range servers {
		out.Servers = append(out.Servers, viewServer(server))
	}
	if out.Servers == nil {
		out.Servers = []serverView{}
	}
	return nil, out, nil
}

type idOrName struct {
	ID   int64  `json:"id,omitempty" jsonschema:"Server id. Use this or name."`
	Name string `json:"name,omitempty" jsonschema:"Server name. Use this or id."`
}

type oneServerOut struct {
	Server serverView `json:"server"`
}

func (a *App) getServer(ctx context.Context, _ *mcp.CallToolRequest, in idOrName) (*mcp.CallToolResult, oneServerOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, oneServerOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, in.Name)
	if err != nil {
		return nil, oneServerOut{}, err
	}
	return nil, oneServerOut{Server: viewServer(server)}, nil
}

type createServerIn struct {
	Name       string            `json:"name" jsonschema:"Hostname, unique in the project."`
	ServerType string            `json:"server_type" jsonschema:"Type name from list_server_types, for example cx23."`
	Image      string            `json:"image" jsonschema:"Image name from list_images, for example ubuntu-24.04."`
	Location   string            `json:"location,omitempty" jsonschema:"Location name from list_locations, for example fsn1. Omit to let Hetzner choose."`
	SSHKeys    []string          `json:"ssh_keys,omitempty" jsonschema:"SSH key names or numeric ids already in the project. Prefer keys over a root password."`
	UserData   string            `json:"user_data,omitempty" jsonschema:"Cloud-init user data. Optional."`
	Labels     map[string]string `json:"labels,omitempty"`
	Wait       *bool             `json:"wait,omitempty" jsonschema:"Wait until creation finishes. Default true."`
}

type createServerOut struct {
	Server       serverView  `json:"server"`
	Action       *actionView `json:"action,omitempty"`
	RootPassword string      `json:"root_password,omitempty" jsonschema:"Set only when no SSH key was used. Show it once."`
}

func (a *App) createServer(ctx context.Context, _ *mcp.CallToolRequest, in createServerIn) (*mcp.CallToolResult, createServerOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, createServerOut{}, err
	}
	serverType, _, err := client.ServerType.Get(ctx, in.ServerType)
	if err != nil || serverType == nil {
		return nil, createServerOut{}, fmt.Errorf("server type %q: %w", in.ServerType, errOrMissing(err))
	}
	image, _, err := client.Image.GetForArchitecture(ctx, in.Image, serverType.Architecture)
	if err != nil || image == nil {
		return nil, createServerOut{}, fmt.Errorf("image %q for %s: %w", in.Image, serverType.Architecture, errOrMissing(err))
	}
	opts := hcloud.ServerCreateOpts{
		Name:       in.Name,
		ServerType: serverType,
		Image:      image,
		UserData:   in.UserData,
		Labels:     in.Labels,
	}
	if in.Location != "" {
		location, _, locErr := client.Location.Get(ctx, in.Location)
		if locErr != nil || location == nil {
			return nil, createServerOut{}, fmt.Errorf("location %q: %w", in.Location, errOrMissing(locErr))
		}
		opts.Location = location
	}
	for _, ref := range in.SSHKeys {
		key, _, keyErr := client.SSHKey.Get(ctx, ref)
		if keyErr != nil || key == nil {
			return nil, createServerOut{}, fmt.Errorf("ssh key %q: %w", ref, errOrMissing(keyErr))
		}
		opts.SSHKeys = append(opts.SSHKeys, key)
	}
	result, _, err := client.Server.Create(ctx, opts)
	if err != nil {
		return nil, createServerOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, result.Action); err != nil {
			return nil, createServerOut{}, err
		}
		if fresh, _, getErr := client.Server.GetByID(ctx, result.Server.ID); getErr == nil && fresh != nil {
			result.Server = fresh
		}
	}
	return nil, createServerOut{
		Server:       viewServer(result.Server),
		Action:       viewAction(result.Action),
		RootPassword: result.RootPassword,
	}, nil
}

type deleteIn struct {
	ID          int64  `json:"id" jsonschema:"Resource id from a get or list tool."`
	ConfirmName string `json:"confirm_name" jsonschema:"Must exactly match the resource name. Read it first."`
}

type deletedOut struct {
	Deleted bool        `json:"deleted"`
	ID      int64       `json:"id"`
	Name    string      `json:"name"`
	Action  *actionView `json:"action,omitempty"`
}

func (a *App) deleteServer(ctx context.Context, _ *mcp.CallToolRequest, in deleteIn) (*mcp.CallToolResult, deletedOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, deletedOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, deletedOut{}, err
	}
	if server.Name != in.ConfirmName {
		return nil, deletedOut{}, fmt.Errorf("confirm_name %q does not match server %q", in.ConfirmName, server.Name)
	}
	if err = refuseProtected("server", server.Name, server.Protection.Delete); err != nil {
		return nil, deletedOut{}, err
	}
	result, _, err := client.Server.DeleteWithResult(ctx, server)
	if err != nil {
		return nil, deletedOut{}, err
	}
	out := deletedOut{Deleted: true, ID: server.ID, Name: server.Name}
	if result != nil {
		out.Action = viewAction(result.Action)
	}
	return nil, out, nil
}

type serverPowerIn struct {
	ID     int64  `json:"id"`
	Action string `json:"action" jsonschema:"shutdown, poweroff, poweron, reboot, or reset. shutdown is graceful. poweroff and reset are hard."`
	Wait   *bool  `json:"wait,omitempty" jsonschema:"Wait until the action finishes. Default true."`
}

type serverPowerOut struct {
	Server serverView `json:"server"`
	Action actionView `json:"action"`
}

func (a *App) serverPower(ctx context.Context, _ *mcp.CallToolRequest, in serverPowerIn) (*mcp.CallToolResult, serverPowerOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, serverPowerOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, serverPowerOut{}, err
	}
	var action *hcloud.Action
	switch in.Action {
	case "shutdown":
		action, _, err = client.Server.Shutdown(ctx, server)
	case "poweroff":
		action, _, err = client.Server.Poweroff(ctx, server)
	case "poweron":
		action, _, err = client.Server.Poweron(ctx, server)
	case "reboot":
		action, _, err = client.Server.Reboot(ctx, server)
	case "reset":
		action, _, err = client.Server.Reset(ctx, server)
	default:
		return nil, serverPowerOut{}, fmt.Errorf("action must be shutdown, poweroff, poweron, reboot, or reset")
	}
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
	return nil, serverPowerOut{Server: viewServer(server), Action: *viewAction(action)}, nil
}

func (a *App) lookupServer(ctx context.Context, client *hcloud.Client, id int64, name string) (*hcloud.Server, error) {
	switch {
	case id != 0:
		server, _, err := client.Server.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if server == nil {
			return nil, fmt.Errorf("server %d not found", id)
		}
		return server, nil
	case name != "":
		server, _, err := client.Server.GetByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if server == nil {
			return nil, fmt.Errorf("server %q not found", name)
		}
		return server, nil
	default:
		return nil, fmt.Errorf("pass id or name")
	}
}

func errOrMissing(err error) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("not found")
}
