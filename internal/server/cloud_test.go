package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListServersUsesTheServerToken(t *testing.T) {
	var sawAuth string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/servers" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("RateLimit-Limit", "3600")
		w.Header().Set("RateLimit-Remaining", "3599")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"servers": []any{serverFixture()},
			"meta": map[string]any{
				"pagination": map[string]any{
					"page": 1, "per_page": 25, "next_page": 2, "last_page": 2, "total_entries": 30,
				},
			},
		})
	}))
	defer api.Close()

	app := &App{Client: hcloud.NewClient(
		hcloud.WithToken("test-token"),
		hcloud.WithEndpoint(api.URL),
		hcloud.WithHTTPClient(api.Client()),
	)}
	session := connect(t, NewServer(app))
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_servers",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool error: %+v", result)
	}
	if sawAuth != "Bearer test-token" {
		t.Fatalf("authorization = %q", sawAuth)
	}
	body, _ := json.Marshal(result.StructuredContent)
	var out listServersOut
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Servers) != 1 || out.Servers[0].Name != "web" || !out.Servers[0].Protection.Delete {
		t.Fatalf("servers %+v", out.Servers)
	}
	if !out.Page.Incomplete || out.Page.NextPage != 2 {
		t.Fatalf("page %+v", out.Page)
	}
}

func connect(t *testing.T, mcpServer *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	if _, err := mcpServer.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func serverFixture() map[string]any {
	return map[string]any{
		"id":      1,
		"name":    "web",
		"status":  "running",
		"created": "2017-08-16T17:29:14+00:00",
		"public_net": map[string]any{
			"ipv4":         map[string]any{"ip": "203.0.113.10", "blocked": false},
			"ipv6":         map[string]any{"ip": "2001:db8::1", "blocked": false, "dns_ptr": []any{}},
			"floating_ips": []any{},
			"firewalls":    []any{},
		},
		"private_net": []any{},
		"server_type": map[string]any{
			"id": 1, "name": "cx23", "description": "cx23", "cores": 2, "memory": 4, "disk": 40,
			"storage_type": "local", "cpu_type": "shared", "architecture": "x86",
		},
		"location": map[string]any{
			"id": 1, "name": "fsn1", "description": "Falkenstein", "country": "DE", "city": "Falkenstein", "network_zone": "eu-central",
		},
		"image": map[string]any{
			"id": 1, "type": "system", "status": "available", "name": "ubuntu-24.04", "os_flavor": "ubuntu", "architecture": "x86",
		},
		"protection":     map[string]any{"delete": true, "rebuild": false},
		"labels":         map[string]any{},
		"locked":         false,
		"backup_window":  nil,
		"rescue_enabled": false,
	}
}
