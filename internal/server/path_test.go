package server

import "testing"

func TestNormalizeCloudPath(t *testing.T) {
	got, err := normalizeCloudPath("/v1/servers/42")
	if err != nil || got != "/servers/42" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err = normalizeCloudPath("https://api.hetzner.cloud/v1/servers"); err == nil {
		t.Fatal("absolute URL should be refused")
	}
	if _, err = normalizeCloudPath("/servers/../firewalls"); err == nil {
		t.Fatal("parent segment should be refused")
	}
}
