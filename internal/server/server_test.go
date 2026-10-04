package server

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolsAdvertiseServerSideAuth(t *testing.T) {
	mcpServer := NewServer(nil)
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
	defer session.Close()

	var names []string
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
		if tool.Name == "cloud_api" {
			props, _ := tool.InputSchema.(map[string]any)["properties"].(map[string]any)
			if _, ok := props["token"]; ok {
				t.Fatal("cloud_api must not accept a token")
			}
			if _, ok := props["authorization"]; ok {
				t.Fatal("cloud_api must not accept an authorization header")
			}
		}
	}
	for _, want := range []string{"find_operations", "cloud_api", "list_servers", "create_server", "delete_server"} {
		if !contains(names, want) {
			t.Fatalf("missing tool %s in %v", want, names)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_servers",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("list_servers without a token should be a tool error")
	}

	found, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "find_operations",
		Arguments: map[string]any{"query": "poweron"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if found.IsError {
		t.Fatalf("catalog search should not need a token: %+v", found)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
