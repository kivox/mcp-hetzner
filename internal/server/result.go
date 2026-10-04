package server

import (
	"net"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type page struct {
	Page          int    `json:"page,omitempty" jsonschema:"1-based page. Pass next_page from the previous result."`
	PerPage       int    `json:"per_page,omitempty" jsonschema:"Page size from 1 to 50. Default 25."`
	LabelSelector string `json:"label_selector,omitempty" jsonschema:"Label selector such as env=prod."`
}

type pageInfo struct {
	Page     int `json:"page,omitempty"`
	PerPage  int `json:"per_page,omitempty"`
	NextPage int `json:"next_page,omitempty"`
	LastPage int `json:"last_page,omitempty"`
	Total    int `json:"total,omitempty"`
}

type rateInfo struct {
	Limit     int `json:"limit,omitempty"`
	Remaining int `json:"remaining,omitempty"`
}

func (p page) listOpts() hcloud.ListOpts {
	per := p.PerPage
	if per <= 0 {
		per = 25
	}
	if per > 50 {
		per = 50
	}
	pageNum := p.Page
	if pageNum <= 0 {
		pageNum = 1
	}
	return hcloud.ListOpts{Page: pageNum, PerPage: per, LabelSelector: p.LabelSelector}
}

func metaOf(resp *hcloud.Response) (pageInfo, rateInfo) {
	if resp == nil {
		return pageInfo{}, rateInfo{}
	}
	var info pageInfo
	if resp.Meta.Pagination != nil {
		pg := resp.Meta.Pagination
		info = pageInfo{
			Page:     pg.Page,
			PerPage:  pg.PerPage,
			NextPage: pg.NextPage,
			LastPage: pg.LastPage,
			Total:    pg.TotalEntries,
		}
	}
	return info, rateInfo{Limit: resp.Meta.Ratelimit.Limit, Remaining: resp.Meta.Ratelimit.Remaining}
}

func wantWait(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

func cidrs(values []string) ([]net.IPNet, error) {
	out := make([]net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, err
		}
		out = append(out, *network)
	}
	return out, nil
}

func ipString(ip net.IP) string {
	if ip == nil || ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}
