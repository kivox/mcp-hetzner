package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/hetznercloud/hcloud-go/v2/hcloud/schema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type isoView struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Type         string `json:"type,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Deprecated   string `json:"deprecated,omitempty"`
}

type isosOut struct {
	ISOs      []isoView `json:"isos"`
	Page      pageInfo  `json:"page"`
	RateLimit rateInfo  `json:"rate_limit"`
}

type listISOsIn struct {
	page
	Name         string `json:"name,omitempty" jsonschema:"Exact ISO name, for example ubuntu-24.04-live-server-amd64.iso."`
	Architecture string `json:"architecture,omitempty" jsonschema:"x86 or arm. Omit for every architecture."`
}

func (a *App) listISOs(ctx context.Context, _ *mcp.CallToolRequest, in listISOsIn) (*mcp.CallToolResult, isosOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, isosOut{}, err
	}
	opts := hcloud.ISOListOpts{ListOpts: in.listOpts(), Name: in.Name}
	if in.Architecture != "" {
		switch hcloud.Architecture(in.Architecture) {
		case hcloud.ArchitectureX86, hcloud.ArchitectureARM:
		default:
			return nil, isosOut{}, fmt.Errorf("architecture must be x86 or arm")
		}
		opts.Architecture = []hcloud.Architecture{hcloud.Architecture(in.Architecture)}
		opts.IncludeWildcardArchitecture = true
	}
	items, resp, err := client.ISO.List(ctx, opts)
	if err != nil {
		return nil, isosOut{}, err
	}
	out := isosOut{}
	out.Page, out.RateLimit = metaOf(resp)
	for _, item := range items {
		view := isoView{ID: item.ID, Name: item.Name, Description: item.Description, Type: string(item.Type)}
		if item.Architecture != nil {
			view.Architecture = string(*item.Architecture)
		}
		if !item.Deprecated.IsZero() {
			view.Deprecated = item.Deprecated.Format("2006-01-02")
		}
		out.ISOs = append(out.ISOs, view)
	}
	if out.ISOs == nil {
		out.ISOs = []isoView{}
	}
	return nil, out, nil
}

type serverISOIn struct {
	ID     int64  `json:"id" jsonschema:"Server id."`
	Action string `json:"action" jsonschema:"attach or detach."`
	ISO    string `json:"iso,omitempty" jsonschema:"ISO name or id. Required for attach."`
	Wait   *bool  `json:"wait,omitempty" jsonschema:"Wait until the action finishes. Default true."`
}

type serverActionOut struct {
	Server serverView `json:"server"`
	Action actionView `json:"action"`
}

// serverISO attaches or detaches a bootable ISO. Hetzner only boots from it on
// the next power cycle, so the reply names the server state.
func (a *App) serverISO(ctx context.Context, _ *mcp.CallToolRequest, in serverISOIn) (*mcp.CallToolResult, serverActionOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, serverActionOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, serverActionOut{}, err
	}
	var action *hcloud.Action
	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "attach":
		if strings.TrimSpace(in.ISO) == "" {
			return nil, serverActionOut{}, fmt.Errorf("iso is required to attach")
		}
		iso, _, isoErr := client.ISO.Get(ctx, in.ISO)
		if isoErr != nil || iso == nil {
			return nil, serverActionOut{}, fmt.Errorf("iso %q: %w", in.ISO, errOrMissing(isoErr))
		}
		action, _, err = client.Server.AttachISO(ctx, server, iso)
	case "detach":
		if strings.TrimSpace(in.ISO) != "" {
			return nil, serverActionOut{}, fmt.Errorf("iso only applies when attaching")
		}
		action, _, err = client.Server.DetachISO(ctx, server)
	default:
		return nil, serverActionOut{}, fmt.Errorf("action must be attach or detach")
	}
	if err != nil {
		return nil, serverActionOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, action); err != nil {
			return nil, serverActionOut{}, err
		}
	}
	if fresh, _, getErr := client.Server.GetByID(ctx, server.ID); getErr == nil && fresh != nil {
		server = fresh
	}
	return nil, serverActionOut{Server: viewServer(server), Action: actionValue(action)}, nil
}

type consoleIn struct {
	ID   int64 `json:"id" jsonschema:"Server id."`
	Wait *bool `json:"wait,omitempty" jsonschema:"Wait until the console action finishes. Default true."`
}

type consoleOut struct {
	Server   serverView  `json:"server"`
	Action   *actionView `json:"action,omitempty"`
	WSSURL   string      `json:"wss_url"`
	Password string      `json:"password" jsonschema:"One-time console password. Show it once."`
}

// requestConsole opens the noVNC console for a server. The URL and password are
// short lived and are handed to the operator, never logged.
func (a *App) requestConsole(ctx context.Context, _ *mcp.CallToolRequest, in consoleIn) (*mcp.CallToolResult, consoleOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, consoleOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, consoleOut{}, err
	}
	result, _, err := client.Server.RequestConsole(ctx, server)
	if err != nil {
		return nil, consoleOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, result.Action); err != nil {
			return nil, consoleOut{}, err
		}
	}
	return nil, consoleOut{
		Server:   viewServer(server),
		Action:   viewAction(result.Action),
		WSSURL:   result.WSSURL,
		Password: result.Password,
	}, nil
}

type backupIn struct {
	ID     int64  `json:"id" jsonschema:"Server id."`
	Action string `json:"action" jsonschema:"enable or disable."`
	Window string `json:"window,omitempty" jsonschema:"Backup window in UTC as HH-HH, for example 22-02. Omit to let Hetzner choose."`
	Wait   *bool  `json:"wait,omitempty" jsonschema:"Wait until the action finishes. Default true."`
}

var backupWindow = regexp.MustCompile(`^([01]\d|2[0-3])-([01]\d|2[0-3])$`)

// validBackupWindow accepts HH-HH with two real hours. A window such as 22-22
// is zero hours long, which Hetzner refuses, so it is rejected here.
func validBackupWindow(window string) bool {
	match := backupWindow.FindStringSubmatch(window)
	if match == nil {
		return false
	}
	return match[1] != match[2]
}

func (a *App) serverBackup(ctx context.Context, _ *mcp.CallToolRequest, in backupIn) (*mcp.CallToolResult, serverActionOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, serverActionOut{}, err
	}
	if in.Window != "" && !validBackupWindow(in.Window) {
		return nil, serverActionOut{}, fmt.Errorf("window must look like 22-02: two different UTC hours")
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, serverActionOut{}, err
	}
	var action *hcloud.Action
	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "enable":
		action, err = enableBackup(ctx, client, server, in.Window)
	case "disable":
		if in.Window != "" {
			return nil, serverActionOut{}, fmt.Errorf("window only applies when enabling backups")
		}
		action, _, err = client.Server.DisableBackup(ctx, server)
	default:
		return nil, serverActionOut{}, fmt.Errorf("action must be enable or disable")
	}
	if err != nil {
		return nil, serverActionOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, action); err != nil {
			return nil, serverActionOut{}, err
		}
	}
	if fresh, _, getErr := client.Server.GetByID(ctx, server.ID); getErr == nil && fresh != nil {
		server = fresh
	}
	return nil, serverActionOut{Server: viewServer(server), Action: actionValue(action)}, nil
}

// enableBackup sends the window itself. hcloud-go v2.50.0 ignores its window
// argument and posts an empty body, which silently drops the requested slot.
func enableBackup(ctx context.Context, client *hcloud.Client, server *hcloud.Server, window string) (*hcloud.Action, error) {
	path := fmt.Sprintf("/servers/%d/actions/enable_backup", server.ID)
	var body io.Reader
	if window != "" {
		body = strings.NewReader(fmt.Sprintf(`{"backup_window":%q}`, window))
	}
	req, err := client.NewRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var out schema.ServerActionEnableBackupResponse
	if _, err = client.Do(req, &out); err != nil {
		return nil, err
	}
	return hcloud.ActionFromSchema(out.Action), nil
}

type rebuildIn struct {
	ID          int64  `json:"id" jsonschema:"Server id."`
	ConfirmName string `json:"confirm_name" jsonschema:"Must exactly match the live server name. Rebuild erases the disk."`
	Image       string `json:"image,omitempty" jsonschema:"Image name from list_images. Defaults to the image the server was built from."`
	UserData    string `json:"user_data,omitempty" jsonschema:"Cloud-init user data for the rebuilt system. Optional."`
	Wait        *bool  `json:"wait,omitempty" jsonschema:"Wait until the rebuild finishes. Default true."`
}

type rebuildOut struct {
	Server       serverView  `json:"server"`
	Action       *actionView `json:"action,omitempty"`
	RootPassword string      `json:"root_password,omitempty" jsonschema:"Set only when the image needed no SSH key. Show it once."`
}

// rebuildServer reinstalls a server from an image. The disk is erased, so the
// caller must repeat the live name and rebuild protection is refused here.
func (a *App) rebuildServer(ctx context.Context, _ *mcp.CallToolRequest, in rebuildIn) (*mcp.CallToolResult, rebuildOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, rebuildOut{}, err
	}
	server, err := a.lookupServer(ctx, client, in.ID, "")
	if err != nil {
		return nil, rebuildOut{}, err
	}
	if server.Name != in.ConfirmName {
		return nil, rebuildOut{}, fmt.Errorf("confirm_name %q does not match server %q", in.ConfirmName, server.Name)
	}
	if server.Protection.Rebuild {
		return nil, rebuildOut{}, fmt.Errorf("server %q has rebuild protection enabled. Disable protection before rebuilding it", server.Name)
	}
	ref := strings.TrimSpace(in.Image)
	if ref == "" {
		if server.Image == nil {
			return nil, rebuildOut{}, fmt.Errorf("this server has no image to fall back to; pass image")
		}
		ref = fmt.Sprintf("%d", server.Image.ID)
	}
	var image *hcloud.Image
	var err2 error
	if server.ServerType != nil && server.ServerType.Architecture != "" {
		image, _, err2 = client.Image.GetForArchitecture(ctx, ref, server.ServerType.Architecture)
	} else {
		image, _, err2 = client.Image.Get(ctx, ref)
	}
	if err2 != nil || image == nil {
		return nil, rebuildOut{}, fmt.Errorf("image %q: %w", ref, errOrMissing(err2))
	}
	opts := hcloud.ServerRebuildOpts{Image: image}
	if in.UserData != "" {
		opts.UserData = &in.UserData
	}
	result, _, err := client.Server.RebuildWithResult(ctx, server, opts)
	if err != nil {
		return nil, rebuildOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, result.Action); err != nil {
			return nil, rebuildOut{}, err
		}
	}
	if fresh, _, getErr := client.Server.GetByID(ctx, server.ID); getErr == nil && fresh != nil {
		server = fresh
	}
	return nil, rebuildOut{
		Server:       viewServer(server),
		Action:       viewAction(result.Action),
		RootPassword: result.RootPassword,
	}, nil
}
