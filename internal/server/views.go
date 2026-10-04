package server

import "github.com/hetznercloud/hcloud-go/v2/hcloud"

type serverView struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	Status   string            `json:"status"`
	Type     string            `json:"type,omitempty"`
	Image    string            `json:"image,omitempty"`
	Location string            `json:"location,omitempty"`
	IPv4     string            `json:"ipv4,omitempty"`
	IPv6     string            `json:"ipv6,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Locked   bool              `json:"locked,omitempty"`
}

type actionView struct {
	ID       int64  `json:"id"`
	Status   string `json:"status"`
	Command  string `json:"command,omitempty"`
	Progress int    `json:"progress,omitempty"`
	Error    string `json:"error,omitempty"`
}

func viewServer(s *hcloud.Server) serverView {
	if s == nil {
		return serverView{}
	}
	view := serverView{
		ID:     s.ID,
		Name:   s.Name,
		Status: string(s.Status),
		Labels: s.Labels,
		Locked: s.Locked,
	}
	if s.ServerType != nil {
		view.Type = s.ServerType.Name
	}
	if s.Image != nil {
		view.Image = s.Image.Name
	}
	if s.Location != nil {
		view.Location = s.Location.Name
	}
	view.IPv4 = ipString(s.PublicNet.IPv4.IP)
	view.IPv6 = ipString(s.PublicNet.IPv6.IP)
	return view
}

func viewAction(a *hcloud.Action) *actionView {
	if a == nil {
		return nil
	}
	view := &actionView{
		ID:       a.ID,
		Status:   string(a.Status),
		Command:  a.Command,
		Progress: a.Progress,
	}
	if a.ErrorMessage != "" {
		view.Error = a.ErrorMessage
	}
	return view
}

func viewActions(actions []*hcloud.Action) []actionView {
	out := make([]actionView, 0, len(actions))
	for _, action := range actions {
		if view := viewAction(action); view != nil {
			out = append(out, *view)
		}
	}
	return out
}

type sshKeyView struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func viewSSHKey(key *hcloud.SSHKey) sshKeyView {
	return sshKeyView{ID: key.ID, Name: key.Name, Fingerprint: key.Fingerprint, Labels: key.Labels}
}

type volumeView struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	Size     int               `json:"size_gb"`
	Status   string            `json:"status,omitempty"`
	Location string            `json:"location,omitempty"`
	ServerID int64             `json:"server_id,omitempty"`
	Device   string            `json:"linux_device,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

func viewVolume(v *hcloud.Volume) volumeView {
	out := volumeView{
		ID: v.ID, Name: v.Name, Size: v.Size, Status: string(v.Status),
		Device: v.LinuxDevice, Labels: v.Labels,
	}
	if v.Location != nil {
		out.Location = v.Location.Name
	}
	if v.Server != nil {
		out.ServerID = v.Server.ID
	}
	return out
}

type firewallRuleView struct {
	Direction      string   `json:"direction"`
	Protocol       string   `json:"protocol"`
	Port           string   `json:"port,omitempty"`
	SourceIPs      []string `json:"source_ips,omitempty"`
	DestinationIPs []string `json:"destination_ips,omitempty"`
	Description    string   `json:"description,omitempty"`
}

type firewallView struct {
	ID     int64              `json:"id"`
	Name   string             `json:"name"`
	Rules  []firewallRuleView `json:"rules,omitempty"`
	Labels map[string]string  `json:"labels,omitempty"`
}

func viewFirewall(fw *hcloud.Firewall) firewallView {
	out := firewallView{ID: fw.ID, Name: fw.Name, Labels: fw.Labels}
	for _, rule := range fw.Rules {
		item := firewallRuleView{
			Direction: string(rule.Direction),
			Protocol:  string(rule.Protocol),
		}
		if rule.Port != nil {
			item.Port = *rule.Port
		}
		if rule.Description != nil {
			item.Description = *rule.Description
		}
		for _, ip := range rule.SourceIPs {
			item.SourceIPs = append(item.SourceIPs, ip.String())
		}
		for _, ip := range rule.DestinationIPs {
			item.DestinationIPs = append(item.DestinationIPs, ip.String())
		}
		out.Rules = append(out.Rules, item)
	}
	return out
}
