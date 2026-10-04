package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/foxcore-cloud/mcp-hetzner/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type findIn struct {
	Query string `json:"query,omitempty" jsonschema:"Words to match, such as 'load balancer target' or 'primary ip'. Empty returns the tag index."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum matches, 1 to 20. Default 8."`
}

type findOut struct {
	Tags       []tagCount          `json:"tags,omitempty"`
	Operations []catalog.Operation `json:"operations,omitempty"`
	Hint       string              `json:"hint,omitempty"`
}

type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func (a *App) findOperations(_ context.Context, _ *mcp.CallToolRequest, in findIn) (*mcp.CallToolResult, findOut, error) {
	if strings.TrimSpace(in.Query) == "" {
		counts, err := catalog.Tags()
		if err != nil {
			return nil, findOut{}, err
		}
		tags := make([]tagCount, 0, len(counts))
		for tag, count := range counts {
			tags = append(tags, tagCount{Tag: tag, Count: count})
		}
		sort.Slice(tags, func(i, j int) bool { return tags[i].Count > tags[j].Count })
		return nil, findOut{
			Tags: tags,
			Hint: "Pass query to narrow this. Then call cloud_api with method and path from operations. The server sends the authenticated request.",
		}, nil
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	ops, err := catalog.Search(in.Query, limit)
	if err != nil {
		return nil, findOut{}, err
	}
	return nil, findOut{
		Operations: ops,
		Hint:       "Call cloud_api with one of these method and path values. Fill path parameters with real ids. Do not send a token.",
	}, nil
}

type cloudIn struct {
	Method string            `json:"method" jsonschema:"GET, POST, PUT, or DELETE."`
	Path   string            `json:"path" jsonschema:"Path copied from find_operations, starting with /. Example: /servers/123/actions/enable_backup. No host and no token."`
	Query  map[string]string `json:"query,omitempty" jsonschema:"Query string parameters."`
	Body   string            `json:"body,omitempty" jsonschema:"Raw JSON body. Omit for GET and DELETE."`
}

type cloudOut struct {
	Status    int             `json:"status"`
	Operation string          `json:"operation,omitempty"`
	Path      string          `json:"path"`
	Body      json.RawMessage `json:"body,omitempty"`
	Text      string          `json:"text,omitempty"`
	RateLimit rateInfo        `json:"rate_limit"`
}

func (a *App) cloudAPI(ctx context.Context, _ *mcp.CallToolRequest, in cloudIn) (*mcp.CallToolResult, cloudOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, cloudOut{}, err
	}
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	path := strings.TrimSpace(in.Path)
	switch method {
	case "GET", "POST", "PUT", "DELETE":
	default:
		return nil, cloudOut{}, fmt.Errorf("method must be GET, POST, PUT, or DELETE")
	}
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.Contains(path, "://") || strings.ContainsAny(path, "?#") {
		return nil, cloudOut{}, fmt.Errorf("path must be an absolute Cloud API path from find_operations, without a query string")
	}
	op, ok, err := catalog.Allow(method, path)
	if err != nil {
		return nil, cloudOut{}, err
	}
	if !ok {
		return nil, cloudOut{}, fmt.Errorf("%s %s is not in the Hetzner Cloud API. Call find_operations", method, path)
	}
	if len(in.Query) > 0 {
		values := url.Values{}
		for key, value := range in.Query {
			values.Set(key, value)
		}
		path += "?" + values.Encode()
	}
	var payload *strings.Reader
	if in.Body != "" {
		if !json.Valid([]byte(in.Body)) {
			return nil, cloudOut{}, fmt.Errorf("body must be JSON")
		}
		payload = strings.NewReader(in.Body)
	}
	req, err := client.NewRequest(ctx, method, path, payload)
	if err != nil {
		return nil, cloudOut{}, err
	}
	var buf bytes.Buffer
	resp, err := client.Do(req, &buf)
	out := cloudOut{Path: op.Path, Operation: op.Operation}
	if resp != nil {
		out.Status = resp.StatusCode
		_, out.RateLimit = metaOf(resp)
	}
	if buf.Len() > 0 && json.Valid(buf.Bytes()) {
		out.Body = json.RawMessage(append([]byte(nil), buf.Bytes()...))
	} else if buf.Len() > 0 {
		out.Text = buf.String()
	}
	if err != nil {
		snippet := buf.String()
		if len(snippet) > 2000 {
			snippet = snippet[:2000]
		}
		return nil, cloudOut{}, fmt.Errorf("%s %s: %w: %s", method, op.Path, err, snippet)
	}
	return nil, out, nil
}

type actionIn struct {
	ID int64 `json:"id" jsonschema:"Action id returned by a previous tool."`
}

func (a *App) getAction(ctx context.Context, _ *mcp.CallToolRequest, in actionIn) (*mcp.CallToolResult, actionView, error) {
	client, err := a.api()
	if err != nil {
		return nil, actionView{}, err
	}
	action, _, err := client.Action.GetByID(ctx, in.ID)
	if err != nil || action == nil {
		return nil, actionView{}, fmt.Errorf("action %d: %w", in.ID, errOrMissing(err))
	}
	return nil, *viewAction(action), nil
}
