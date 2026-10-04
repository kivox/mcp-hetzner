package server

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type firewallsOut struct {
	Firewalls []firewallView `json:"firewalls"`
	Page      pageInfo       `json:"page"`
}

func (a *App) listFirewalls(ctx context.Context, _ *mcp.CallToolRequest, in namedList) (*mcp.CallToolResult, firewallsOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, firewallsOut{}, err
	}
	items, resp, err := client.Firewall.List(ctx, hcloud.FirewallListOpts{ListOpts: in.listOpts(), Name: in.Name})
	if err != nil {
		return nil, firewallsOut{}, err
	}
	out := firewallsOut{}
	out.Page, _ = metaOf(resp)
	for _, item := range items {
		out.Firewalls = append(out.Firewalls, viewFirewall(item))
	}
	if out.Firewalls == nil {
		out.Firewalls = []firewallView{}
	}
	return nil, out, nil
}

type firewallRef struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func (a *App) getFirewall(ctx context.Context, _ *mcp.CallToolRequest, in firewallRef) (*mcp.CallToolResult, firewallView, error) {
	client, err := a.api()
	if err != nil {
		return nil, firewallView{}, err
	}
	fw, err := lookupFirewall(ctx, client, in.ID, in.Name)
	if err != nil {
		return nil, firewallView{}, err
	}
	return nil, viewFirewall(fw), nil
}

type ruleIn struct {
	Direction      string   `json:"direction" jsonschema:"in or out."`
	Protocol       string   `json:"protocol" jsonschema:"tcp, udp, icmp, esp, or gre."`
	Port           string   `json:"port,omitempty" jsonschema:"Required for tcp and udp. A port or range such as 80 or 8000-9000."`
	SourceIPs      []string `json:"source_ips,omitempty" jsonschema:"CIDR sources. Required for inbound rules."`
	DestinationIPs []string `json:"destination_ips,omitempty" jsonschema:"CIDR destinations. Required for outbound rules."`
	Description    string   `json:"description,omitempty"`
}

type createFirewallIn struct {
	Name   string            `json:"name"`
	Rules  []ruleIn          `json:"rules,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

func (a *App) createFirewall(ctx context.Context, _ *mcp.CallToolRequest, in createFirewallIn) (*mcp.CallToolResult, firewallView, error) {
	client, err := a.api()
	if err != nil {
		return nil, firewallView{}, err
	}
	rules, err := rulesFrom(in.Rules)
	if err != nil {
		return nil, firewallView{}, err
	}
	result, _, err := client.Firewall.Create(ctx, hcloud.FirewallCreateOpts{Name: in.Name, Labels: in.Labels, Rules: rules})
	if err != nil {
		return nil, firewallView{}, err
	}
	return nil, viewFirewall(result.Firewall), nil
}

type setRulesIn struct {
	ID    int64    `json:"id"`
	Rules []ruleIn `json:"rules" jsonschema:"Replaces every rule. An empty list removes them all."`
}

type actionsOut struct {
	Actions []actionView `json:"actions"`
}

func (a *App) setFirewallRules(ctx context.Context, _ *mcp.CallToolRequest, in setRulesIn) (*mcp.CallToolResult, actionsOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, actionsOut{}, err
	}
	fw, err := lookupFirewall(ctx, client, in.ID, "")
	if err != nil {
		return nil, actionsOut{}, err
	}
	rules, err := rulesFrom(in.Rules)
	if err != nil {
		return nil, actionsOut{}, err
	}
	actions, _, err := client.Firewall.SetRules(ctx, fw, hcloud.FirewallSetRulesOpts{Rules: rules})
	if err != nil {
		return nil, actionsOut{}, err
	}
	return nil, actionsOut{Actions: viewActions(actions)}, nil
}

type applyFirewallIn struct {
	ID            int64  `json:"id"`
	ServerID      int64  `json:"server_id,omitempty" jsonschema:"Apply to this server. Use this or label_selector."`
	LabelSelector string `json:"label_selector,omitempty" jsonschema:"Apply to servers matching this selector."`
	Remove        bool   `json:"remove,omitempty" jsonschema:"Remove the firewall from the target instead of applying it."`
}

func (a *App) applyFirewall(ctx context.Context, _ *mcp.CallToolRequest, in applyFirewallIn) (*mcp.CallToolResult, actionsOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, actionsOut{}, err
	}
	fw, err := lookupFirewall(ctx, client, in.ID, "")
	if err != nil {
		return nil, actionsOut{}, err
	}
	var resource hcloud.FirewallResource
	switch {
	case in.ServerID != 0:
		server, lookErr := a.lookupServer(ctx, client, in.ServerID, "")
		if lookErr != nil {
			return nil, actionsOut{}, lookErr
		}
		resource = hcloud.FirewallResource{Type: hcloud.FirewallResourceTypeServer, Server: &hcloud.FirewallResourceServer{ID: server.ID}}
	case in.LabelSelector != "":
		resource = hcloud.FirewallResource{
			Type:          hcloud.FirewallResourceTypeLabelSelector,
			LabelSelector: &hcloud.FirewallResourceLabelSelector{Selector: in.LabelSelector},
		}
	default:
		return nil, actionsOut{}, fmt.Errorf("pass server_id or label_selector")
	}
	var actions []*hcloud.Action
	if in.Remove {
		actions, _, err = client.Firewall.RemoveResources(ctx, fw, []hcloud.FirewallResource{resource})
	} else {
		actions, _, err = client.Firewall.ApplyResources(ctx, fw, []hcloud.FirewallResource{resource})
	}
	if err != nil {
		return nil, actionsOut{}, err
	}
	return nil, actionsOut{Actions: viewActions(actions)}, nil
}

func (a *App) deleteFirewall(ctx context.Context, _ *mcp.CallToolRequest, in deleteIn) (*mcp.CallToolResult, deletedOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, deletedOut{}, err
	}
	fw, err := lookupFirewall(ctx, client, in.ID, "")
	if err != nil {
		return nil, deletedOut{}, err
	}
	if fw.Name != in.ConfirmName {
		return nil, deletedOut{}, fmt.Errorf("confirm_name %q does not match firewall %q", in.ConfirmName, fw.Name)
	}
	if _, err = client.Firewall.Delete(ctx, fw); err != nil {
		return nil, deletedOut{}, err
	}
	return nil, deletedOut{Deleted: true, ID: fw.ID, Name: fw.Name}, nil
}

func lookupFirewall(ctx context.Context, client *hcloud.Client, id int64, name string) (*hcloud.Firewall, error) {
	switch {
	case id != 0:
		fw, _, err := client.Firewall.GetByID(ctx, id)
		if err != nil || fw == nil {
			return nil, fmt.Errorf("firewall %d: %w", id, errOrMissing(err))
		}
		return fw, nil
	case name != "":
		fw, _, err := client.Firewall.GetByName(ctx, name)
		if err != nil || fw == nil {
			return nil, fmt.Errorf("firewall %q: %w", name, errOrMissing(err))
		}
		return fw, nil
	default:
		return nil, fmt.Errorf("pass id or name")
	}
}

func rulesFrom(in []ruleIn) ([]hcloud.FirewallRule, error) {
	rules := make([]hcloud.FirewallRule, 0, len(in))
	for _, rule := range in {
		item := hcloud.FirewallRule{
			Direction: hcloud.FirewallRuleDirection(rule.Direction),
			Protocol:  hcloud.FirewallRuleProtocol(rule.Protocol),
		}
		if rule.Port != "" {
			item.Port = &rule.Port
		}
		if rule.Description != "" {
			item.Description = &rule.Description
		}
		var err error
		if len(rule.SourceIPs) > 0 {
			item.SourceIPs, err = cidrs(rule.SourceIPs)
			if err != nil {
				return nil, fmt.Errorf("source_ips: %w", err)
			}
		}
		if len(rule.DestinationIPs) > 0 {
			item.DestinationIPs, err = cidrs(rule.DestinationIPs)
			if err != nil {
				return nil, fmt.Errorf("destination_ips: %w", err)
			}
		}
		rules = append(rules, item)
	}
	return rules, nil
}
