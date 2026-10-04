package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/foxcore-cloud/hetzner-mcp/internal/catalog"
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
			Hint: "Pass query to narrow this. Use cloud_get for a GET path and cloud_api for POST, PUT, or DELETE. The server sends the authenticated request.",
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
		Hint:       "Use cloud_get when method is GET. Use cloud_api for POST, PUT, and DELETE. Copy the path and fill ids. Do not send a token.",
	}, nil
}

const maxCloudBody = 256 << 10

type cloudGetIn struct {
	Path  string            `json:"path" jsonschema:"GET path copied from find_operations, starting with /. Example: /networks. No host and no token."`
	Query map[string]string `json:"query,omitempty" jsonschema:"Query parameters such as page and per_page."`
}

type cloudIn struct {
	Method string            `json:"method" jsonschema:"POST, PUT, or DELETE. Use cloud_get for GET."`
	Path   string            `json:"path" jsonschema:"Path copied from find_operations, starting with /. Example: /servers/123/actions/enable_backup. No host and no token."`
	Query  map[string]string `json:"query,omitempty" jsonschema:"Query string parameters."`
	Body   string            `json:"body,omitempty" jsonschema:"Raw JSON body, at most 256KB. Omit when the operation has no body."`
}

type cloudOut struct {
	Status    int             `json:"status"`
	Operation string          `json:"operation,omitempty"`
	Path      string          `json:"path"`
	Body      json.RawMessage `json:"body,omitempty"`
	Text      string          `json:"text,omitempty"`
	Truncated bool            `json:"truncated,omitempty"`
	RateLimit rateInfo        `json:"rate_limit"`
	Error     string          `json:"error,omitempty"`
}

// normalizeCloudPath accepts the paths find_operations returns and the /v1
// form agents copy from the Hetzner docs. The client already targets /v1.
func normalizeCloudPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.Contains(path, "://") || strings.ContainsAny(path, "?#") {
		return "", fmt.Errorf("path must be an absolute Cloud API path from find_operations, without a query string")
	}
	return path, nil
}

func (a *App) cloudGet(ctx context.Context, _ *mcp.CallToolRequest, in cloudGetIn) (*mcp.CallToolResult, cloudOut, error) {
	return a.callCloud(ctx, "GET", in.Path, in.Query, "")
}

func (a *App) cloudAPI(ctx context.Context, _ *mcp.CallToolRequest, in cloudIn) (*mcp.CallToolResult, cloudOut, error) {
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	switch method {
	case "POST", "PUT", "DELETE":
	case "GET":
		return nil, cloudOut{}, fmt.Errorf("use cloud_get for GET")
	default:
		return nil, cloudOut{}, fmt.Errorf("method must be POST, PUT, or DELETE")
	}
	return a.callCloud(ctx, method, in.Path, in.Query, in.Body)
}

func (a *App) callCloud(ctx context.Context, method, path string, query map[string]string, body string) (*mcp.CallToolResult, cloudOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, cloudOut{}, err
	}
	path, err = normalizeCloudPath(path)
	if err != nil {
		return nil, cloudOut{}, err
	}
	op, ok, err := catalog.Allow(method, path)
	if err != nil {
		return nil, cloudOut{}, err
	}
	if !ok {
		return nil, cloudOut{}, fmt.Errorf("%s %s is not in the Hetzner Cloud API. Call find_operations", method, path)
	}
	if len(body) > maxCloudBody {
		return nil, cloudOut{}, fmt.Errorf("body is %d bytes; the limit is %d", len(body), maxCloudBody)
	}
	requestPath := path
	if len(query) > 0 {
		values := url.Values{}
		for key, value := range query {
			values.Set(key, value)
		}
		requestPath += "?" + values.Encode()
	}
	var payload *strings.Reader
	if body != "" {
		if !json.Valid([]byte(body)) {
			return nil, cloudOut{}, fmt.Errorf("body must be JSON")
		}
		payload = strings.NewReader(body)
	}
	httpReq, err := client.NewRequest(ctx, method, requestPath, payload)
	if err != nil {
		return nil, cloudOut{}, err
	}
	var buf bytes.Buffer
	resp, err := client.Do(httpReq, &buf)
	out := cloudOut{Path: op.Path, Operation: op.Operation}
	if resp != nil {
		out.Status = resp.StatusCode
		_, out.RateLimit = metaOf(resp)
	}
	payloadBytes := buf.Bytes()
	if len(payloadBytes) > maxCloudBody {
		out.Truncated = true
		out.Error = "response is larger than 256KB. Repeat the call with query page and per_page."
		out.Text = string(payloadBytes[:1024])
		return &mcp.CallToolResult{IsError: true}, out, nil
	}
	if len(payloadBytes) > 0 && json.Valid(payloadBytes) {
		out.Body = json.RawMessage(append([]byte(nil), payloadBytes...))
	} else if len(payloadBytes) > 0 {
		out.Text = string(payloadBytes)
	}
	if err != nil {
		out.Error = err.Error()
		return &mcp.CallToolResult{IsError: true}, out, nil
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
