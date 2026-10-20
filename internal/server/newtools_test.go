package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLabelPatchModes(t *testing.T) {
	current := map[string]string{"env": "prod", "team": "ops"}

	got, changed, err := labelPatch{AddLabels: map[string]string{"tier": "web"}}.apply(current)
	if err != nil || !changed {
		t.Fatalf("merge: changed=%v err=%v", changed, err)
	}
	if len(got) != 3 || got["env"] != "prod" || got["tier"] != "web" {
		t.Fatalf("merge result %v", got)
	}
	if _, ok := current["tier"]; ok {
		t.Fatal("apply must not mutate the current label map")
	}

	got, changed, err = labelPatch{RemoveLabelKeys: []string{"team"}}.apply(current)
	if err != nil || !changed || len(got) != 1 || got["env"] != "prod" {
		t.Fatalf("remove result %v changed=%v err=%v", got, changed, err)
	}

	got, changed, err = labelPatch{Labels: map[string]string{}}.apply(current)
	if err != nil || !changed || len(got) != 0 {
		t.Fatalf("replace with empty map result %v changed=%v err=%v", got, changed, err)
	}

	if _, _, err = (labelPatch{Labels: map[string]string{"a": "b"}, AddLabels: map[string]string{"c": "d"}}).apply(current); err == nil {
		t.Fatal("two modes at once must be refused")
	}
	if _, changed, err = (labelPatch{}).apply(current); changed || err != nil {
		t.Fatalf("no label fields: changed=%v err=%v", changed, err)
	}
	if _, _, err = (labelPatch{AddLabels: map[string]string{"": "x"}}).apply(current); err == nil {
		t.Fatal("an empty label key must be refused")
	}
}

func TestUpdateServerMergesLabels(t *testing.T) {
	var putBody map[string]any
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/servers/1":
			fixture := serverFixture()
			fixture["labels"] = map[string]any{"env": "prod"}
			writeJSON(w, map[string]any{"server": fixture})
		case r.Method == http.MethodPut && r.URL.Path == "/servers/1":
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Errorf("decode put body: %v", err)
			}
			fixture := serverFixture()
			fixture["labels"] = map[string]any{"env": "prod", "tier": "web"}
			writeJSON(w, map[string]any{"server": fixture})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "update_server", map[string]any{
		"id":         1,
		"add_labels": map[string]any{"tier": "web"},
	})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	labels, ok := putBody["labels"].(map[string]any)
	if !ok || labels["env"] != "prod" || labels["tier"] != "web" {
		t.Fatalf("labels sent = %v", putBody["labels"])
	}

	conflict := callTool(t, session, "update_server", map[string]any{
		"id":                1,
		"labels":            map[string]any{"a": "b"},
		"remove_label_keys": []any{"env"},
	})
	if !conflict.IsError {
		t.Fatal("a replacement and a removal in one call must be a tool error")
	}
}

func TestUpdateFirewallAndSSHKey(t *testing.T) {
	var firewallPut map[string]any
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/firewalls/3":
			writeJSON(w, map[string]any{
				"firewall": map[string]any{
					"id": 3, "name": "edge", "labels": map[string]any{"env": "prod"},
					"rules": []any{}, "applied_to": []any{}, "created": "2026-10-20T00:00:00+00:00",
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/firewalls/3":
			_ = json.NewDecoder(r.Body).Decode(&firewallPut)
			writeJSON(w, map[string]any{
				"firewall": map[string]any{
					"id": 3, "name": "edge2", "labels": map[string]any{"env": "prod"},
					"rules": []any{}, "applied_to": []any{}, "created": "2026-10-20T00:00:00+00:00",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/ssh_keys/4":
			writeJSON(w, map[string]any{
				"ssh_key": map[string]any{"id": 4, "name": "deploy", "fingerprint": "aa:bb", "labels": map[string]any{}},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/ssh_keys/4":
			writeJSON(w, map[string]any{
				"ssh_key": map[string]any{"id": 4, "name": "deploy2", "fingerprint": "aa:bb", "labels": map[string]any{}},
			})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	fw := callTool(t, session, "update_firewall", map[string]any{"id": 3, "name": "edge2"})
	if fw.IsError {
		t.Fatalf("firewall update: %+v", fw)
	}
	if firewallPut["name"] != "edge2" {
		t.Fatalf("firewall put body %v", firewallPut)
	}
	var fwOut firewallView
	structured(t, fw, &fwOut)
	if fwOut.Name != "edge2" {
		t.Fatalf("firewall name %q", fwOut.Name)
	}

	key := callTool(t, session, "update_ssh_key", map[string]any{"id": 4, "name": "deploy2"})
	if key.IsError {
		t.Fatalf("ssh key update: %+v", key)
	}
	var keyOut sshKeyView
	structured(t, key, &keyOut)
	if keyOut.Name != "deploy2" {
		t.Fatalf("ssh key name %q", keyOut.Name)
	}
}

func TestListServerActions(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/servers/42":
			fixture := serverFixture()
			fixture["id"] = 42
			writeJSON(w, map[string]any{"server": fixture})
		case "/servers/42/actions":
			if got := r.URL.Query().Get("status"); got != "success" {
				t.Errorf("status filter = %q", got)
			}
			if got := r.URL.Query().Get("page"); got != "1" {
				t.Errorf("page = %q", got)
			}
			writeJSON(w, map[string]any{
				"actions": []any{actionFixture(9, "success")},
				"meta": map[string]any{
					"pagination": map[string]any{
						"page": 1, "per_page": 25, "next_page": 2, "last_page": 2, "total_entries": 2,
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "list_server_actions", map[string]any{"id": 42, "status": "success"})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	var out actionsPageOut
	structured(t, result, &out)
	if len(out.Actions) != 1 || out.Actions[0].ID != 9 || out.Actions[0].Status != "success" {
		t.Fatalf("actions %+v", out.Actions)
	}
	if out.Page.NextPage != 2 || !out.Page.Incomplete {
		t.Fatalf("page %+v", out.Page)
	}

	noIDs := callTool(t, session, "list_actions", map[string]any{})
	if !noIDs.IsError {
		t.Fatal("list_actions without ids must be a tool error")
	}
}

func TestEnableRescue(t *testing.T) {
	const oneTimePassword = "one-time-password"
	var rescueBody map[string]any
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/servers/7":
			fixture := serverFixture()
			fixture["id"] = 7
			fixture["status"] = "off"
			writeJSON(w, map[string]any{"server": fixture})
		case r.Method == http.MethodPost && r.URL.Path == "/servers/7/actions/enable_rescue":
			_ = json.NewDecoder(r.Body).Decode(&rescueBody)
			writeJSON(w, map[string]any{
				"action":        actionFixture(11, "success"),
				"root_password": "one-time-password",
			})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "enable_server_rescue", map[string]any{"id": 7})
	if result.IsError {
		t.Fatalf("tool error: %+v: %s", result, toolText(result))
	}
	var out rescueOut
	structured(t, result, &out)
	if out.RootPassword != "one-time-password" {
		t.Fatalf("root password %q", out.RootPassword)
	}
	if out.Action == nil || out.Action.ID != 11 {
		t.Fatalf("action %+v", out.Action)
	}
	if rescueBody["type"] != "linux64" {
		t.Fatalf("rescue body %v", rescueBody)
	}
	if out.Server.Status != "off" {
		t.Fatalf("server status %q", out.Server.Status)
	}
}

func TestEnableRescueRefusesRunningServer(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/servers/7" {
			fixture := serverFixture()
			fixture["id"] = 7
			writeJSON(w, map[string]any{"server": fixture})
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	session := connect(t, NewServer(app))

	result := callTool(t, session, "enable_server_rescue", map[string]any{"id": 7})
	if !result.IsError {
		t.Fatal("rescue on a running server must be a tool error")
	}
	text := toolText(result)
	if !strings.Contains(text, "shutdown") {
		t.Fatalf("error should name the next step: %q", text)
	}
}

func TestGetAndUpdateVolume(t *testing.T) {
	app := fakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		volume := map[string]any{
			"id": 9, "name": "data", "size": 10, "status": "available",
			"linux_device": "/dev/disk/by-id/scsi-0HC_Volume_9",
			"location":     map[string]any{"id": 1, "name": "fsn1"},
			"protection":   map[string]any{"delete": false},
			"labels":       map[string]any{"env": "prod"},
			"created":      "2026-10-20T00:00:00+00:00",
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/volumes/9":
			writeJSON(w, map[string]any{"volume": volume})
		case r.Method == http.MethodPut && r.URL.Path == "/volumes/9":
			volume["name"] = "data2"
			writeJSON(w, map[string]any{"volume": volume})
		default:
			http.NotFound(w, r)
		}
	})
	session := connect(t, NewServer(app))

	got := callTool(t, session, "get_volume", map[string]any{"id": 9})
	if got.IsError {
		t.Fatalf("get_volume: %+v", got)
	}
	var out volumeOut
	structured(t, got, &out)
	if out.Volume.Name != "data" || out.Volume.Location != "fsn1" {
		t.Fatalf("volume %+v", out.Volume)
	}

	updated := callTool(t, session, "update_volume", map[string]any{"id": 9, "name": "data2"})
	if updated.IsError {
		t.Fatalf("update_volume: %+v", updated)
	}
	structured(t, updated, &out)
	if out.Volume.Name != "data2" {
		t.Fatalf("updated volume %+v", out.Volume)
	}
}

func TestNewToolsAreAdvertised(t *testing.T) {
	session := connect(t, NewServer(nil))
	want := []string{
		"list_actions", "list_server_actions",
		"enable_server_rescue", "disable_server_rescue",
		"update_firewall", "update_ssh_key",
		"get_volume", "update_volume",
	}
	have := map[string]*mcp.Tool{}
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		have[tool.Name] = tool
	}
	for _, name := range want {
		tool, ok := have[name]
		if !ok {
			t.Fatalf("missing tool %s", name)
		}
		if tool.Annotations == nil || tool.Annotations.Title == "" {
			t.Fatalf("%s has no title annotation", name)
		}
	}
	for _, name := range []string{"list_actions", "list_server_actions"} {
		if hint := have[name].Annotations.ReadOnlyHint; !hint {
			t.Fatalf("%s must be read-only", name)
		}
	}
}

func fakeApp(t *testing.T, handler http.HandlerFunc) *App {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	return &App{Client: hcloud.NewClient(
		hcloud.WithToken("test-token"),
		hcloud.WithEndpoint(api.URL),
		hcloud.WithHTTPClient(api.Client()),
	)}
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return result
}

func structured(t *testing.T, result *mcp.CallToolResult, dest any) {
	t.Helper()
	body, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatal(err)
	}
}

func toolText(result *mcp.CallToolResult) string {
	var parts []string
	for _, item := range result.Content {
		if text, ok := item.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func actionFixture(id int64, status string) map[string]any {
	return map[string]any{
		"id": id, "command": "start_server", "status": status, "progress": 100,
		"started": "2026-10-20T00:00:00+00:00", "finished": "2026-10-20T00:00:01+00:00",
		"resources": []any{}, "error": nil,
	}
}

func TestActionValueHandlesMissingAction(t *testing.T) {
	if got := actionValue(nil); got != (actionView{}) {
		t.Fatalf("nil action = %+v", got)
	}
}
