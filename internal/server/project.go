package server

import (
	"context"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type countView struct {
	Total      int  `json:"total"`
	Incomplete bool `json:"incomplete,omitempty"`
}

type projectOut struct {
	Servers       []serverView `json:"servers"`
	ServerPage    pageInfo     `json:"server_page"`
	Volumes       countView    `json:"volumes"`
	Firewalls     countView    `json:"firewalls"`
	Networks      countView    `json:"networks"`
	LoadBalancers countView    `json:"load_balancers"`
	SSHKeys       countView    `json:"ssh_keys"`
	PrimaryIPs    countView    `json:"primary_ips"`
	FloatingIPs   countView    `json:"floating_ips"`
	RateLimit     rateInfo     `json:"rate_limit"`
}

func (a *App) describeProject(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, projectOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, projectOut{}, err
	}
	var out projectOut
	servers, resp, err := client.Server.List(ctx, hcloud.ServerListOpts{ListOpts: hcloud.ListOpts{Page: 1, PerPage: 25}})
	if err != nil {
		return nil, projectOut{}, err
	}
	out.ServerPage, out.RateLimit = metaOf(resp)
	for _, server := range servers {
		out.Servers = append(out.Servers, viewServer(server))
	}
	if out.Servers == nil {
		out.Servers = []serverView{}
	}
	counts := []struct {
		dest *countView
		load func() (*hcloud.Response, error)
	}{
		{&out.Volumes, func() (*hcloud.Response, error) {
			_, resp, err := client.Volume.List(ctx, hcloud.VolumeListOpts{ListOpts: hcloud.ListOpts{Page: 1, PerPage: 1}})
			return resp, err
		}},
		{&out.Firewalls, func() (*hcloud.Response, error) {
			_, resp, err := client.Firewall.List(ctx, hcloud.FirewallListOpts{ListOpts: hcloud.ListOpts{Page: 1, PerPage: 1}})
			return resp, err
		}},
		{&out.Networks, func() (*hcloud.Response, error) {
			_, resp, err := client.Network.List(ctx, hcloud.NetworkListOpts{ListOpts: hcloud.ListOpts{Page: 1, PerPage: 1}})
			return resp, err
		}},
		{&out.LoadBalancers, func() (*hcloud.Response, error) {
			_, resp, err := client.LoadBalancer.List(ctx, hcloud.LoadBalancerListOpts{ListOpts: hcloud.ListOpts{Page: 1, PerPage: 1}})
			return resp, err
		}},
		{&out.SSHKeys, func() (*hcloud.Response, error) {
			_, resp, err := client.SSHKey.List(ctx, hcloud.SSHKeyListOpts{ListOpts: hcloud.ListOpts{Page: 1, PerPage: 1}})
			return resp, err
		}},
		{&out.PrimaryIPs, func() (*hcloud.Response, error) {
			_, resp, err := client.PrimaryIP.List(ctx, hcloud.PrimaryIPListOpts{ListOpts: hcloud.ListOpts{Page: 1, PerPage: 1}})
			return resp, err
		}},
		{&out.FloatingIPs, func() (*hcloud.Response, error) {
			_, resp, err := client.FloatingIP.List(ctx, hcloud.FloatingIPListOpts{ListOpts: hcloud.ListOpts{Page: 1, PerPage: 1}})
			return resp, err
		}},
	}
	for _, item := range counts {
		resp, err := item.load()
		if err != nil {
			return nil, projectOut{}, err
		}
		page, rate := metaOf(resp)
		item.dest.Total = page.Total
		item.dest.Incomplete = page.Incomplete
		if rate.SlowDown {
			out.RateLimit = rate
		}
	}
	return nil, out, nil
}
