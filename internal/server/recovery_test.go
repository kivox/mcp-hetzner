package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func isoFixture() map[string]any {
	return map[string]any{
		"id": 101, "name": "ubuntu-24.04-live-server-amd64.iso", "description": "Ubuntu 24.04",
		"type": "public", "architecture": "x86", "deprecated": nil,
	}
}

func TestListISOs(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/isos" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("architecture"); got != "x86" {
			t.Errorf("architecture = %q", got)
		}
		writeJSON(w, map[string]any{
			"isos": []any{isoFixture()},
			"meta": map[string]any{"pagination": map[string]any{
				"page": 1, "per_page": 25, "next_page": 0, "last_page": 1, "total_entries": 1,
			}},
		})
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "list_isos", map[string]any{"architecture": "x86"})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	var out isosOut
	structured(t, result, &out)
	if len(out.ISOs) != 1 || out.ISOs[0].Name != "ubuntu-24.04-live-server-amd64.iso" || out.ISOs[0].Architecture != "x86" {
		t.Fatalf("isos %+v", out.ISOs)
	}
	if out.Page.Total != 1 || out.Page.Incomplete {
		t.Fatalf("page %+v", out.Page)
	}
}

func TestServerISOAttachAndDetach(t *testing.T) {
	var attachBody map[string]any
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/servers/1":
			writeJSON(w, map[string]any{"server": serverFixture()})
		case r.Method == http.MethodGet && r.URL.Path == "/isos":
			writeJSON(w, map[string]any{"isos": []any{isoFixture()}})
		case r.Method == http.MethodPost && r.URL.Path == "/servers/1/actions/attach_iso":
			_ = json.NewDecoder(r.Body).Decode(&attachBody)
			writeJSON(w, map[string]any{"action": actionFixture(20, "success")})
		case r.Method == http.MethodPost && r.URL.Path == "/servers/1/actions/detach_iso":
			writeJSON(w, map[string]any{"action": actionFixture(21, "success")})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	attached := callTool(t, session, "server_iso", map[string]any{
		"id": 1, "action": "attach", "iso": "ubuntu-24.04-live-server-amd64.iso",
	})
	if attached.IsError {
		t.Fatalf("attach: %+v: %s", attached, toolText(attached))
	}
	if attachBody["iso"] != float64(101) {
		t.Fatalf("attach body %v", attachBody)
	}
	var out serverActionOut
	structured(t, attached, &out)
	if out.Action.ID != 20 || out.Server.Name != "web" {
		t.Fatalf("attach out %+v", out)
	}

	detached := callTool(t, session, "server_iso", map[string]any{"id": 1, "action": "detach"})
	if detached.IsError {
		t.Fatalf("detach: %+v: %s", detached, toolText(detached))
	}
	structured(t, detached, &out)
	if out.Action.ID != 21 {
		t.Fatalf("detach action %+v", out.Action)
	}
}

func TestRequestConsole(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/servers/1":
			writeJSON(w, map[string]any{"server": serverFixture()})
		case r.Method == http.MethodPost && r.URL.Path == "/servers/1/actions/request_console":
			writeJSON(w, map[string]any{
				"action":   actionFixture(30, "success"),
				"wss_url":  "wss://console.hetzner.cloud/?token=abc",
				"password": "console-pass",
			})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "request_console", map[string]any{"id": 1})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	var out consoleOut
	structured(t, result, &out)
	if out.WSSURL == "" || out.Password != "console-pass" {
		t.Fatalf("console out %+v", out)
	}
	if out.Action == nil || out.Action.ID != 30 {
		t.Fatalf("action %+v", out.Action)
	}
}

// EnableBackup posts no request body: Hetzner dropped the backup window from
// the endpoint (hcloud-go CHANGELOG 2.14.0, hcloud-go#525), so there is nothing
// to send and the tool no longer accepts a window.
func TestServerBackupEnable(t *testing.T) {
	var (
		sawContentType string
		sawBody        string
	)
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/servers/1":
			writeJSON(w, map[string]any{"server": serverFixture()})
		case r.Method == http.MethodPost && r.URL.Path == "/servers/1/actions/enable_backup":
			sawContentType = r.Header.Get("Content-Type")
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
			}
			sawBody = string(body)
			writeJSON(w, map[string]any{"action": actionFixture(40, "success")})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "server_backup", map[string]any{"id": 1, "action": "enable"})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	if sawContentType != "" {
		t.Fatalf("enable_backup must not send a content type, got %q", sawContentType)
	}
	if sawBody != "" {
		t.Fatalf("enable_backup must send an empty body, got %q", sawBody)
	}
	var out serverActionOut
	structured(t, result, &out)
	if out.Action.ID != 40 {
		t.Fatalf("action %+v", out.Action)
	}
}

func TestRebuildServer(t *testing.T) {
	var rebuildBody map[string]any
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/servers/1":
			writeJSON(w, map[string]any{"server": serverFixture()})
		case r.Method == http.MethodGet && r.URL.Path == "/images":
			if got := r.URL.Query().Get("name"); got != "ubuntu-24.04" {
				t.Errorf("image name = %q", got)
			}
			writeJSON(w, map[string]any{"images": []any{
				map[string]any{"id": 9, "type": "system", "name": "ubuntu-24.04", "architecture": "x86", "os_flavor": "ubuntu"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/servers/1/actions/rebuild":
			_ = json.NewDecoder(r.Body).Decode(&rebuildBody)
			writeJSON(w, map[string]any{"action": actionFixture(50, "success"), "root_password": "rebuild-pass"})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "rebuild_server", map[string]any{
		"id": 1, "confirm_name": "web", "image": "ubuntu-24.04",
	})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	if rebuildBody["image"] != float64(9) {
		t.Fatalf("rebuild body %v", rebuildBody)
	}
	var out rebuildOut
	structured(t, result, &out)
	if out.RootPassword != "rebuild-pass" || out.Action == nil || out.Action.ID != 50 {
		t.Fatalf("rebuild out %+v", out)
	}

	wrongName := callTool(t, session, "rebuild_server", map[string]any{"id": 1, "confirm_name": "other"})
	if !wrongName.IsError {
		t.Fatal("a mismatched confirm_name must be refused")
	}
}

func TestRebuildRefusesProtection(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/servers/1" {
			fixture := serverFixture()
			fixture["protection"] = map[string]any{"delete": false, "rebuild": true}
			writeJSON(w, map[string]any{"server": fixture})
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "rebuild_server", map[string]any{"id": 1, "confirm_name": "web"})
	if !result.IsError {
		t.Fatal("rebuild on a protected server must be a tool error")
	}
	if text := toolText(result); !strings.Contains(text, "protection") {
		t.Fatalf("error should name protection: %q", text)
	}
}

func TestServerNetworkAttachAndDetach(t *testing.T) {
	var attachBody, detachBody map[string]any
	network := map[string]any{
		"id": 5, "name": "core", "ip_range": "10.0.0.0/8", "subnets": []any{},
		"routes": []any{}, "servers": []any{}, "protection": map[string]any{"delete": false},
		"labels": map[string]any{}, "created": "2026-10-20T00:00:00+00:00",
	}
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/servers/1":
			writeJSON(w, map[string]any{"server": serverFixture()})
		case r.Method == http.MethodGet && r.URL.Path == "/networks/5":
			writeJSON(w, map[string]any{"network": network})
		case r.Method == http.MethodPost && r.URL.Path == "/servers/1/actions/attach_to_network":
			_ = json.NewDecoder(r.Body).Decode(&attachBody)
			writeJSON(w, map[string]any{"action": actionFixture(60, "success")})
		case r.Method == http.MethodPost && r.URL.Path == "/servers/1/actions/detach_from_network":
			_ = json.NewDecoder(r.Body).Decode(&detachBody)
			writeJSON(w, map[string]any{"action": actionFixture(61, "success")})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	attached := callTool(t, session, "server_network", map[string]any{
		"id": 1, "action": "attach", "network": "5", "ip": "10.0.0.5",
		"alias_ips": []any{"10.0.0.6"},
	})
	if attached.IsError {
		t.Fatalf("attach: %+v: %s", attached, toolText(attached))
	}
	if attachBody["network"] != float64(5) || attachBody["ip"] != "10.0.0.5" {
		t.Fatalf("attach body %v", attachBody)
	}
	aliases, _ := attachBody["alias_ips"].([]any)
	if len(aliases) != 1 || aliases[0] != "10.0.0.6" {
		t.Fatalf("alias_ips %v", attachBody["alias_ips"])
	}

	detached := callTool(t, session, "server_network", map[string]any{"id": 1, "action": "detach", "network": "5"})
	if detached.IsError {
		t.Fatalf("detach: %+v: %s", detached, toolText(detached))
	}
	if detachBody["network"] != float64(5) {
		t.Fatalf("detach body %v", detachBody)
	}

	bad := callTool(t, session, "server_network", map[string]any{
		"id": 1, "action": "detach", "network": "5", "ip": "10.0.0.5",
	})
	if !bad.IsError {
		t.Fatal("an ip on detach must be refused")
	}
}

func TestFirewallAppliedToVisible(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/firewalls" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"firewalls": []any{
			map[string]any{
				"id": 3, "name": "edge", "labels": map[string]any{}, "rules": []any{},
				"applied_to": []any{
					map[string]any{"type": "server", "server": map[string]any{"id": 1}},
					map[string]any{
						"type": "label_selector", "label_selector": map[string]any{"selector": "env=prod"},
						"applied_to_resources": []any{map[string]any{"type": "server", "server": map[string]any{"id": 2}}},
					},
				},
			},
		}})
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "list_firewalls", map[string]any{})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	var out firewallsOut
	structured(t, result, &out)
	if len(out.Firewalls) != 1 || len(out.Firewalls[0].AppliedTo) != 2 {
		t.Fatalf("applied_to %+v", out.Firewalls)
	}
	targets := out.Firewalls[0].AppliedTo
	if targets[0].ServerID != 1 || targets[1].LabelSelector != "env=prod" {
		t.Fatalf("targets %+v", targets)
	}
	if len(targets[1].ServerIDs) != 1 || targets[1].ServerIDs[0] != 2 {
		t.Fatalf("selector servers %+v", targets[1])
	}
}

func TestFirewallRuleValidation(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	session := connect(t, NewServer(app))

	cases := []struct {
		name string
		rule map[string]any
		hint string
	}{
		{"no source", map[string]any{"direction": "in", "protocol": "tcp", "port": "22"}, "source_ips"},
		{"no port", map[string]any{"direction": "in", "protocol": "tcp", "source_ips": []any{"0.0.0.0/0"}}, "port"},
		{"port on icmp", map[string]any{"direction": "in", "protocol": "icmp", "port": "1", "source_ips": []any{"0.0.0.0/0"}}, "port"},
		{"bad direction", map[string]any{"direction": "sideways", "protocol": "tcp", "port": "22"}, "direction"},
		{"bad cidr", map[string]any{"direction": "in", "protocol": "tcp", "port": "22", "source_ips": []any{"nope"}}, "source_ips"},
	}
	for _, tc := range cases {
		result := callTool(t, session, "create_firewall", map[string]any{
			"name": "edge", "rules": []any{tc.rule},
		})
		if !result.IsError {
			t.Fatalf("%s: expected a tool error", tc.name)
		}
		if text := toolText(result); !strings.Contains(text, tc.hint) {
			t.Fatalf("%s: error %q does not mention %q", tc.name, text, tc.hint)
		}
	}
}

func TestApplyFirewallRefusesTwoTargets(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "apply_firewall", map[string]any{
		"id": 3, "server_id": 1, "label_selector": "env=prod",
	})
	if !result.IsError {
		t.Fatal("two targets in one call must be refused")
	}
}

func TestVolumeValidation(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/volumes/9":
			writeJSON(w, map[string]any{"volume": map[string]any{
				"id": 9, "name": "data", "size": 10, "status": "available",
				"location": map[string]any{"id": 1, "name": "fsn1"}, "labels": map[string]any{},
				"protection": map[string]any{"delete": false}, "created": "2026-10-20T00:00:00+00:00",
			}})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	noTarget := callTool(t, session, "create_volume", map[string]any{"name": "data", "size": 10})
	if !noTarget.IsError {
		t.Fatal("a volume without a location or server must be refused")
	}
	tooSmall := callTool(t, session, "create_volume", map[string]any{"name": "data", "size": 5, "location": "fsn1"})
	if !tooSmall.IsError {
		t.Fatal("a volume under 10 GB must be refused")
	}
	shrink := callTool(t, session, "volume_action", map[string]any{"id": 9, "action": "resize", "size": 5})
	if !shrink.IsError {
		t.Fatal("shrinking a volume must be refused")
	}
	if text := toolText(shrink); !strings.Contains(text, "cannot shrink") {
		t.Fatalf("shrink error %q", text)
	}
}

func TestListFiltersAreValidated(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	session := connect(t, NewServer(app))

	if result := callTool(t, session, "list_servers", map[string]any{"status": "sideways"}); !result.IsError {
		t.Fatal("an unknown server status must be refused")
	}
	if result := callTool(t, session, "list_images", map[string]any{"type": "sideways"}); !result.IsError {
		t.Fatal("an unknown image type must be refused")
	}
	if result := callTool(t, session, "list_images", map[string]any{"architecture": "mips"}); !result.IsError {
		t.Fatal("an unknown architecture must be refused")
	}
}

func TestCreateToolsAreNotIdempotent(t *testing.T) {
	session := connect(t, NewServer(nil))
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Name != "create_server" && tool.Name != "create_volume" && tool.Name != "create_firewall" && tool.Name != "create_ssh_key" {
			continue
		}
		annotations := tool.Annotations
		if annotations == nil {
			t.Fatalf("%s has no annotations", tool.Name)
		}
		if annotations.IdempotentHint {
			t.Fatalf("%s is a create and must not be marked idempotent", tool.Name)
		}
		if annotations.DestructiveHint == nil || *annotations.DestructiveHint {
			t.Fatalf("%s must be non-destructive", tool.Name)
		}
	}
}

// Every request_console call mints a new one-time token and password, so it is
// not idempotent and must not be advertised as safe to repeat.
func TestRequestConsoleIsNotIdempotent(t *testing.T) {
	session := connect(t, NewServer(nil))
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Name != "request_console" {
			continue
		}
		if tool.Annotations == nil {
			t.Fatal("request_console has no annotations")
		}
		if tool.Annotations.IdempotentHint {
			t.Fatal("request_console mints a new token on every call and must not be marked idempotent")
		}
		return
	}
	t.Fatal("request_console is not registered")
}

func TestEveryToolIsAnnotated(t *testing.T) {
	session := connect(t, NewServer(nil))
	seen := map[string]bool{}
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if seen[tool.Name] {
			t.Fatalf("duplicate tool %s", tool.Name)
		}
		seen[tool.Name] = true
		if tool.Description == "" {
			t.Fatalf("%s has no description", tool.Name)
		}
		if tool.Annotations == nil || tool.Annotations.Title == "" {
			t.Fatalf("%s has no title annotation", tool.Name)
		}
	}
	if len(seen) < 40 {
		t.Fatalf("only %d tools registered", len(seen))
	}
}
