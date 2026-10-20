package main

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strings"
)

func loopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// guardPublicListen refuses to publish the Hetzner credential on a
// non-loopback address unless the MCP endpoint itself requires a bearer token.
func guardPublicListen(host, mcpToken string) error {
	if loopbackHost(host) || mcpToken != "" {
		return nil
	}
	return fmt.Errorf("refusing to listen on %s without MCP_AUTH_TOKEN", host)
}

func requireBearer(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "Bearer " + token
		got := r.Header.Get("Authorization")
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
