package server

import (
	"net/http"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

const version = "0.2.0"

// NewClient returns the HTTP client every tool shares.
// The timeout bounds one request. Action polling issues a new request per tick,
// so a long create is not cut off by this deadline. hcloud-go already retries
// rate limits and transient failures.
func NewClient(token string) *hcloud.Client {
	return hcloud.NewClient(
		hcloud.WithToken(token),
		hcloud.WithApplication("mcp-hetzner", version),
		hcloud.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	)
}
