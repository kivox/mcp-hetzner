package server

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type serverNetworkIn struct {
	ID      int64    `json:"id" jsonschema:"Server id."`
	Action  string   `json:"action" jsonschema:"attach or detach."`
	Network string   `json:"network" jsonschema:"Network id or name from list_networks."`
	IP      string   `json:"ip,omitempty" jsonschema:"Private IP to assign on attach. Omit to let Hetzner pick one."`
	AliasIP []string `json:"alias_ips,omitempty" jsonschema:"Extra private IPs to assign on attach."`
	Wait    *bool    `json:"wait,omitempty" jsonschema:"Wait until the action finishes. Default true."`
}

// serverNetwork attaches a server to a private network or detaches it. The
// network must already exist; create one with cloud_api.
func (a *App) serverNetwork(ctx context.Context, _ *mcp.CallToolRequest, in serverNetworkIn) (*mcp.CallToolResult, serverActionOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, serverActionOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, serverActionOut{}, err
	}
	ref := strings.TrimSpace(in.Network)
	if ref == "" {
		return nil, serverActionOut{}, fmt.Errorf("network is required")
	}
	network, _, err := client.Network.Get(ctx, ref)
	if err != nil || network == nil {
		return nil, serverActionOut{}, fmt.Errorf("network %q: %w", ref, errOrMissing(err))
	}
	var action *hcloud.Action
	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "attach":
		opts := hcloud.ServerAttachToNetworkOpts{Network: network}
		if in.IP != "" {
			ip := net.ParseIP(in.IP)
			if ip == nil {
				return nil, serverActionOut{}, fmt.Errorf("ip %q is not an IP address", in.IP)
			}
			opts.IP = ip
		}
		for _, value := range in.AliasIP {
			ip := net.ParseIP(value)
			if ip == nil {
				return nil, serverActionOut{}, fmt.Errorf("alias_ips: %q is not an IP address", value)
			}
			opts.AliasIPs = append(opts.AliasIPs, ip)
		}
		action, _, err = client.Server.AttachToNetwork(ctx, server, opts)
	case "detach":
		if in.IP != "" || len(in.AliasIP) > 0 {
			return nil, serverActionOut{}, fmt.Errorf("ip and alias_ips only apply when attaching")
		}
		action, _, err = client.Server.DetachFromNetwork(ctx, server, hcloud.ServerDetachFromNetworkOpts{Network: network})
	default:
		return nil, serverActionOut{}, fmt.Errorf("action must be attach or detach")
	}
	if err != nil {
		return nil, serverActionOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, action); err != nil {
			return nil, serverActionOut{}, err
		}
	}
	if fresh, _, getErr := client.Server.GetByID(ctx, server.ID); getErr == nil && fresh != nil {
		server = fresh
	}
	return nil, serverActionOut{Server: viewServer(server), Action: actionValue(action)}, nil
}
