package main

import "testing"

func TestGuardPublicListen(t *testing.T) {
	if err := guardPublicListen("127.0.0.1", ""); err != nil {
		t.Fatal(err)
	}
	if err := guardPublicListen("::1", ""); err != nil {
		t.Fatal(err)
	}
	if err := guardPublicListen("0.0.0.0", ""); err == nil {
		t.Fatal("a public listen without MCP_AUTH_TOKEN must be refused")
	}
	if err := guardPublicListen("0.0.0.0", "secret"); err != nil {
		t.Fatal(err)
	}
}
