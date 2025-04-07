// Package catalog is the agent-facing index of the Hetzner Cloud OpenAPI spec.
// It is generated from https://docs.hetzner.cloud/cloud.spec.json so a model
// can find any operation without loading all 190 tools into its prompt.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed operations.json
var raw []byte

// Operation is one Cloud API call.
type Operation struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Tag       string `json:"tag"`
	Summary   string `json:"summary"`
}

var (
	loadOnce sync.Once
	all      []Operation
	loadErr  error
	matchers []matcher
)

type matcher struct {
	op Operation
	re *regexp.Regexp
}

// All returns every operation in the spec.
func All() ([]Operation, error) {
	if err := load(); err != nil {
		return nil, err
	}
	return all, nil
}

// Tags returns operation counts grouped by spec tag, sorted by count.
func Tags() (map[string]int, error) {
	ops, err := All()
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, op := range ops {
		out[op.Tag]++
	}
	return out, nil
}

// Search ranks operations by a free-text query. An empty query returns nothing;
// callers should show tags instead of the whole spec.
func Search(query string, limit int) ([]Operation, error) {
	ops, err := All()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || limit <= 0 {
		return nil, nil
	}
	terms := strings.Fields(query)
	type hit struct {
		op    Operation
		score int
	}
	var hits []hit
	for _, op := range ops {
		hay := strings.ToLower(op.Tag + " " + op.Operation + " " + op.Method + " " + op.Path + " " + op.Summary)
		score := 0
		ok := true
		for _, term := range terms {
			if !strings.Contains(hay, term) {
				ok = false
				break
			}
			score += strings.Count(hay, term)
			if strings.Contains(strings.ToLower(op.Path), term) {
				score += 3
			}
			if strings.Contains(strings.ToLower(op.Operation), term) {
				score += 2
			}
		}
		if ok {
			hits = append(hits, hit{op, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].op.Path < hits[j].op.Path
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Operation, len(hits))
	for i, h := range hits {
		out[i] = h.op
	}
	return out, nil
}

// Allow reports whether method and path are in the spec.
// The path must be absolute, without a query string or a scheme.
func Allow(method, path string) (Operation, bool, error) {
	if err := load(); err != nil {
		return Operation{}, false, err
	}
	method = strings.ToUpper(method)
	for _, m := range matchers {
		if m.op.Method == method && m.re.MatchString(path) {
			return m.op, true, nil
		}
	}
	return Operation{}, false, nil
}

func load() error {
	loadOnce.Do(func() {
		if err := json.Unmarshal(raw, &all); err != nil {
			loadErr = fmt.Errorf("catalog: %w", err)
			return
		}
		matchers = make([]matcher, 0, len(all))
		for _, op := range all {
			matchers = append(matchers, matcher{op: op, re: pathRegexp(op.Path)})
		}
	})
	return loadErr
}

func pathRegexp(tmpl string) *regexp.Regexp {
	var b strings.Builder
	b.WriteByte('^')
	for _, part := range strings.Split(strings.Trim(tmpl, "/"), "/") {
		b.WriteByte('/')
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			b.WriteString(`[^/]+`)
			continue
		}
		b.WriteString(regexp.QuoteMeta(part))
	}
	b.WriteByte('$')
	return regexp.MustCompile(b.String())
}
