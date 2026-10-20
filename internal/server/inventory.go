package server

import (
	"context"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type networkView struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IPRange   string `json:"ip_range,omitempty"`
	Servers   int    `json:"servers"`
	Subnets   int    `json:"subnets"`
	Protected bool   `json:"delete_protection,omitempty"`
}

type networksOut struct {
	Networks  []networkView `json:"networks"`
	Page      pageInfo      `json:"page"`
	RateLimit rateInfo      `json:"rate_limit"`
}

func (a *App) listNetworks(ctx context.Context, _ *mcp.CallToolRequest, in namedList) (*mcp.CallToolResult, networksOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, networksOut{}, err
	}
	items, resp, err := client.Network.List(ctx, hcloud.NetworkListOpts{ListOpts: in.listOpts(), Name: in.Name})
	if err != nil {
		return nil, networksOut{}, err
	}
	out := networksOut{}
	out.Page, out.RateLimit = metaOf(resp)
	for _, item := range items {
		view := networkView{ID: item.ID, Name: item.Name, Servers: len(item.Servers), Subnets: len(item.Subnets), Protected: item.Protection.Delete}
		if item.IPRange != nil {
			view.IPRange = item.IPRange.String()
		}
		out.Networks = append(out.Networks, view)
	}
	if out.Networks == nil {
		out.Networks = []networkView{}
	}
	return nil, out, nil
}

type loadBalancerView struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Location  string `json:"location,omitempty"`
	IPv4      string `json:"ipv4,omitempty"`
	IPv6      string `json:"ipv6,omitempty"`
	Services  int    `json:"services"`
	Targets   int    `json:"targets"`
	Protected bool   `json:"delete_protection,omitempty"`
}

type loadBalancersOut struct {
	LoadBalancers []loadBalancerView `json:"load_balancers"`
	Page          pageInfo           `json:"page"`
	RateLimit     rateInfo           `json:"rate_limit"`
}

func (a *App) listLoadBalancers(ctx context.Context, _ *mcp.CallToolRequest, in namedList) (*mcp.CallToolResult, loadBalancersOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, loadBalancersOut{}, err
	}
	items, resp, err := client.LoadBalancer.List(ctx, hcloud.LoadBalancerListOpts{ListOpts: in.listOpts(), Name: in.Name})
	if err != nil {
		return nil, loadBalancersOut{}, err
	}
	out := loadBalancersOut{}
	out.Page, out.RateLimit = metaOf(resp)
	for _, item := range items {
		view := loadBalancerView{
			ID: item.ID, Name: item.Name, Services: len(item.Services), Targets: len(item.Targets),
			Protected: item.Protection.Delete,
			IPv4:      ipString(item.PublicNet.IPv4.IP),
			IPv6:      ipString(item.PublicNet.IPv6.IP),
		}
		if item.LoadBalancerType != nil {
			view.Type = item.LoadBalancerType.Name
		}
		if item.Location != nil {
			view.Location = item.Location.Name
		}
		out.LoadBalancers = append(out.LoadBalancers, view)
	}
	if out.LoadBalancers == nil {
		out.LoadBalancers = []loadBalancerView{}
	}
	return nil, out, nil
}

type addressView struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IP        string `json:"ip,omitempty"`
	Type      string `json:"type,omitempty"`
	Assignee  int64  `json:"assignee_id,omitempty"`
	Location  string `json:"location,omitempty"`
	Blocked   bool   `json:"blocked,omitempty"`
	Protected bool   `json:"delete_protection,omitempty"`
}

type addressesOut struct {
	Addresses []addressView `json:"addresses"`
	Page      pageInfo      `json:"page"`
	RateLimit rateInfo      `json:"rate_limit"`
}

func (a *App) listPrimaryIPs(ctx context.Context, _ *mcp.CallToolRequest, in namedList) (*mcp.CallToolResult, addressesOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, addressesOut{}, err
	}
	items, resp, err := client.PrimaryIP.List(ctx, hcloud.PrimaryIPListOpts{ListOpts: in.listOpts(), Name: in.Name})
	if err != nil {
		return nil, addressesOut{}, err
	}
	out := addressesOut{}
	out.Page, _ = metaOf(resp)
	for _, item := range items {
		view := addressView{
			ID: item.ID, Name: item.Name, IP: ipString(item.IP), Type: string(item.Type),
			Assignee: item.AssigneeID, Blocked: item.Blocked, Protected: item.Protection.Delete,
		}
		if item.Location != nil {
			view.Location = item.Location.Name
		}
		out.Addresses = append(out.Addresses, view)
	}
	if out.Addresses == nil {
		out.Addresses = []addressView{}
	}
	return nil, out, nil
}

func (a *App) listFloatingIPs(ctx context.Context, _ *mcp.CallToolRequest, in namedList) (*mcp.CallToolResult, addressesOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, addressesOut{}, err
	}
	items, resp, err := client.FloatingIP.List(ctx, hcloud.FloatingIPListOpts{ListOpts: in.listOpts(), Name: in.Name})
	if err != nil {
		return nil, addressesOut{}, err
	}
	out := addressesOut{}
	out.Page, _ = metaOf(resp)
	for _, item := range items {
		view := addressView{
			ID: item.ID, Name: item.Name, IP: ipString(item.IP), Type: string(item.Type),
			Blocked: item.Blocked, Protected: item.Protection.Delete,
		}
		if item.Server != nil {
			view.Assignee = item.Server.ID
		}
		if item.HomeLocation != nil {
			view.Location = item.HomeLocation.Name
		}
		out.Addresses = append(out.Addresses, view)
	}
	if out.Addresses == nil {
		out.Addresses = []addressView{}
	}
	return nil, out, nil
}
