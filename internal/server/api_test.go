package server

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// A bodyless cloud_api call must not hand a typed-nil reader to hcloud:
// http.NewRequestWithContext dereferences a *strings.Reader to set
// ContentLength, so a typed nil panics before the request is sent.
func TestCloudAPIBodylessCall(t *testing.T) {
	var (
		sawPath        string
		sawContentType string
		sawBody        string
	)
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/servers/1/actions/reboot" {
			http.NotFound(w, r)
			return
		}
		sawPath = r.URL.Path
		sawContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		sawBody = string(body)
		writeJSON(w, map[string]any{"action": actionFixture(60, "success")})
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "cloud_api", map[string]any{
		"method": "POST",
		"path":   "/servers/1/actions/reboot",
	})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	if sawPath != "/servers/1/actions/reboot" {
		t.Fatalf("path = %q", sawPath)
	}
	if sawContentType != "" {
		t.Fatalf("a bodyless call must not send a content type, got %q", sawContentType)
	}
	if sawBody != "" {
		t.Fatalf("a bodyless call must send an empty body, got %q", sawBody)
	}
	var out cloudOut
	structured(t, result, &out)
	if out.Status != http.StatusOK || out.Operation != "reboot_server" {
		t.Fatalf("cloud out %+v", out)
	}
	body, ok := out.Body.(map[string]any)
	if !ok {
		t.Fatalf("object response body %T %+v", out.Body, out.Body)
	}
	if _, ok := body["action"]; !ok {
		t.Fatalf("object response body %+v", body)
	}
}

// A cloud_api call with a body must still send it as JSON.
func TestCloudAPIForwardsBody(t *testing.T) {
	var sawBody map[string]any
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/servers/1" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&sawBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		writeJSON(w, map[string]any{"server": serverFixture()})
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "cloud_api", map[string]any{
		"method": "PUT",
		"path":   "/servers/1",
		"body":   `{"name":"renamed"}`,
	})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	if sawBody["name"] != "renamed" {
		t.Fatalf("forwarded body %v", sawBody)
	}
}

// The IP list tools must report the rate-limit meta like every other list.
func TestIPListsReportRateLimit(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("RateLimit-Limit", "3600")
		w.Header().Set("RateLimit-Remaining", "12")
		switch r.URL.Path {
		case "/primary_ips":
			writeJSON(w, map[string]any{"primary_ips": []any{map[string]any{
				"id": 1, "name": "primary", "ip": "203.0.113.5", "type": "ipv4",
				"blocked": false, "protection": map[string]any{"delete": false},
				"location": map[string]any{"name": "fsn1"},
			}}})
		case "/floating_ips":
			writeJSON(w, map[string]any{"floating_ips": []any{map[string]any{
				"id": 2, "name": "floating", "ip": "203.0.113.6", "type": "ipv4",
				"blocked": false, "protection": map[string]any{"delete": false},
				"home_location": map[string]any{"name": "fsn1"},
			}}})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	for _, tool := range []string{"list_primary_ips", "list_floating_ips"} {
		result := callTool(t, session, tool, map[string]any{})
		if result.IsError {
			t.Fatalf("%s: %+v: %s", tool, result, toolText(result))
		}
		var out addressesOut
		structured(t, result, &out)
		if len(out.Addresses) != 1 {
			t.Fatalf("%s addresses %+v", tool, out.Addresses)
		}
		if out.RateLimit.Limit != 3600 || out.RateLimit.Remaining != 12 || !out.RateLimit.SlowDown {
			t.Fatalf("%s rate limit %+v", tool, out.RateLimit)
		}
	}
}
