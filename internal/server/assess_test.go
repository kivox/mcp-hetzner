package server

import (
	"net/http"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func TestIncompletePageAndSlowDown(t *testing.T) {
	resp := &hcloud.Response{
		Response: &http.Response{},
		Meta: hcloud.Meta{
			Pagination: &hcloud.Pagination{Page: 1, PerPage: 25, NextPage: 2, LastPage: 3, TotalEntries: 60},
			Ratelimit:  hcloud.Ratelimit{Limit: 3600, Remaining: 5},
		},
	}
	page, rate := metaOf(resp)
	if !page.Incomplete || page.NextPage != 2 || page.Total != 60 {
		t.Fatalf("page %+v", page)
	}
	if !rate.SlowDown || rate.Remaining != 5 {
		t.Fatalf("rate %+v", rate)
	}
	quiet, _ := metaOf(&hcloud.Response{Response: &http.Response{}, Meta: hcloud.Meta{Ratelimit: hcloud.Ratelimit{Limit: 3600, Remaining: 3000}}})
	if quiet.Incomplete {
		t.Fatal("a finished page was marked incomplete")
	}
}

func TestRefuseProtected(t *testing.T) {
	if err := refuseProtected("server", "web", false); err != nil {
		t.Fatal(err)
	}
	err := refuseProtected("server", "web", true)
	if err == nil || err.Error() == "" {
		t.Fatal("expected a protection error")
	}
}
