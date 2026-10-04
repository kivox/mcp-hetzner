package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrefersEnv(t *testing.T) {
	t.Setenv("HCLOUD_TOKEN", "from-env")
	t.Setenv("HCLOUD_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	token, source, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if token != "from-env" || source != SourceEnv {
		t.Fatalf("token source = %s %q", source, token)
	}
}

func TestResolveReadsActiveCLIContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cli.toml")
	body := `
active_context = "prod"

[[contexts]]
name = "prod"
token = "prod-token"

[[contexts]]
name = "dev"
token = "dev-token"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("HCLOUD_CONTEXT", "")
	t.Setenv("HCLOUD_CONFIG", path)
	token, source, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if token != "prod-token" || source != SourceContext+" prod" {
		t.Fatalf("got %s %q", source, token)
	}

	t.Setenv("HCLOUD_CONTEXT", "dev")
	token, _, err = Resolve()
	if err != nil || token != "dev-token" {
		t.Fatalf("context override: %v %q", err, token)
	}
}
