package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/foxcore-cloud/mcp-hetzner/internal/auth"
	"github.com/foxcore-cloud/mcp-hetzner/internal/server"
)

func main() {
	transport := flag.String("transport", "stdio", "stdio or http. http is the streamable MCP endpoint.")
	host := flag.String("host", env("MCP_HOST", "127.0.0.1"), "HTTP listen host")
	port := flag.Int("port", envInt("MCP_PORT", 8080), "HTTP listen port")
	flag.Parse()

	app := &server.App{}
	token, source, err := auth.Resolve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-hetzner: %s\n", err)
		fmt.Fprintf(os.Stderr, "mcp-hetzner: tools that call Hetzner will fail until a token is available\n")
	} else {
		app.Client = hcloud.NewClient(hcloud.WithToken(token))
		fmt.Fprintf(os.Stderr, "mcp-hetzner: authenticated via %s\n", source)
	}

	mcpServer := server.NewServer(app)
	switch *transport {
	case "stdio":
		if err := mcpServer.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
	case "http", "sse":
		addr := fmt.Sprintf("%s:%d", *host, *port)
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return mcpServer }, nil)
		fmt.Fprintf(os.Stderr, "mcp-hetzner: streamable HTTP on http://%s\n", addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown transport %q", *transport)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscan(value, &parsed); err != nil {
		return fallback
	}
	return parsed
}
