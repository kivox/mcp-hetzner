package catalog

import (
	"strings"
	"testing"
)

func TestAllowMatchesTemplates(t *testing.T) {
	op, ok, err := Allow("GET", "/servers")
	if err != nil || !ok {
		t.Fatalf("list servers: ok=%v err=%v", ok, err)
	}
	if op.Path != "/servers" || op.Method != "GET" {
		t.Fatalf("unexpected op %+v", op)
	}
	op, ok, err = Allow("POST", "/servers/42/actions/poweron")
	if err != nil || !ok {
		t.Fatalf("poweron: ok=%v err=%v", ok, err)
	}
	if op.Method != "POST" {
		t.Fatalf("method %s", op.Method)
	}
	_, ok, err = Allow("DELETE", "/not-a-real-resource")
	if err != nil || ok {
		t.Fatalf("unknown path should be refused, ok=%v err=%v", ok, err)
	}
	_, ok, err = Allow("GET", "https://evil.example/servers")
	if err != nil || ok {
		t.Fatalf("absolute URL should be refused")
	}
}

func TestSearchRequiresEveryTerm(t *testing.T) {
	ops, err := Search("firewall rules", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 {
		t.Fatal("expected firewall rule operations")
	}
	for _, op := range ops {
		hay := strings.ToLower(op.Tag + op.Path + op.Summary + op.Operation)
		if !strings.Contains(hay, "firewall") {
			t.Fatalf("result %s does not mention firewall", op.Path)
		}
	}
	empty, err := Search("", 10)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty query should not dump the spec: %v %d", err, len(empty))
	}
}

func TestCatalogCoversTheSpec(t *testing.T) {
	ops, err := All()
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) < 180 {
		t.Fatalf("catalog shrank to %d operations", len(ops))
	}
}
