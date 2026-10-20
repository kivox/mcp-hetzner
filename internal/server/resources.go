package server

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type namedList struct {
	page
	Name string `json:"name,omitempty" jsonschema:"Exact name filter when the API supports it."`
}

type serverTypeView struct {
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	Cores        int     `json:"cores"`
	MemoryGB     float32 `json:"memory_gb"`
	DiskGB       int     `json:"disk_gb"`
	CPU          string  `json:"cpu_type,omitempty"`
	Architecture string  `json:"architecture,omitempty"`
	Storage      string  `json:"storage_type,omitempty"`
}

type serverTypesOut struct {
	ServerTypes []serverTypeView `json:"server_types"`
	Page        pageInfo         `json:"page"`
	RateLimit   rateInfo         `json:"rate_limit"`
}

func (a *App) listServerTypes(ctx context.Context, _ *mcp.CallToolRequest, in page) (*mcp.CallToolResult, serverTypesOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, serverTypesOut{}, err
	}
	items, resp, err := client.ServerType.List(ctx, hcloud.ServerTypeListOpts{ListOpts: in.listOpts()})
	if err != nil {
		return nil, serverTypesOut{}, err
	}
	out := serverTypesOut{}
	out.Page, out.RateLimit = metaOf(resp)
	for _, item := range items {
		out.ServerTypes = append(out.ServerTypes, serverTypeView{
			Name: item.Name, Description: item.Description, Cores: item.Cores,
			MemoryGB: item.Memory, DiskGB: item.Disk, CPU: string(item.CPUType),
			Architecture: string(item.Architecture), Storage: string(item.StorageType),
		})
	}
	if out.ServerTypes == nil {
		out.ServerTypes = []serverTypeView{}
	}
	return nil, out, nil
}

type imageView struct {
	ID           int64  `json:"id"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	Type         string `json:"type,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	OS           string `json:"os,omitempty"`
}

type imagesOut struct {
	Images    []imageView `json:"images"`
	Page      pageInfo    `json:"page"`
	RateLimit rateInfo    `json:"rate_limit"`
}

type listImagesIn struct {
	page
	Type         string `json:"type,omitempty" jsonschema:"system, snapshot, backup, or app. Default system."`
	Architecture string `json:"architecture,omitempty" jsonschema:"x86 or arm."`
	Name         string `json:"name,omitempty"`
}

func (a *App) listImages(ctx context.Context, _ *mcp.CallToolRequest, in listImagesIn) (*mcp.CallToolResult, imagesOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, imagesOut{}, err
	}
	kind := in.Type
	if kind == "" {
		kind = "system"
	}
	switch hcloud.ImageType(kind) {
	case hcloud.ImageTypeSystem, hcloud.ImageTypeSnapshot, hcloud.ImageTypeBackup, hcloud.ImageTypeApp:
	default:
		return nil, imagesOut{}, fmt.Errorf("type must be system, snapshot, backup, or app")
	}
	opts := hcloud.ImageListOpts{ListOpts: in.listOpts(), Type: []hcloud.ImageType{hcloud.ImageType(kind)}, Name: in.Name}
	if in.Architecture != "" {
		switch hcloud.Architecture(in.Architecture) {
		case hcloud.ArchitectureX86, hcloud.ArchitectureARM:
		default:
			return nil, imagesOut{}, fmt.Errorf("architecture must be x86 or arm")
		}
		opts.Architecture = []hcloud.Architecture{hcloud.Architecture(in.Architecture)}
	}
	items, resp, err := client.Image.List(ctx, opts)
	if err != nil {
		return nil, imagesOut{}, err
	}
	out := imagesOut{}
	out.Page, out.RateLimit = metaOf(resp)
	for _, item := range items {
		out.Images = append(out.Images, imageView{
			ID: item.ID, Name: item.Name, Description: item.Description,
			Type: string(item.Type), Architecture: string(item.Architecture),
			OS: item.OSFlavor,
		})
	}
	if out.Images == nil {
		out.Images = []imageView{}
	}
	return nil, out, nil
}

type locationView struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Country     string `json:"country,omitempty"`
	City        string `json:"city,omitempty"`
	NetworkZone string `json:"network_zone,omitempty"`
}

type locationsOut struct {
	Locations []locationView `json:"locations"`
}

func (a *App) listLocations(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, locationsOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, locationsOut{}, err
	}
	items, err := client.Location.All(ctx)
	if err != nil {
		return nil, locationsOut{}, err
	}
	out := locationsOut{}
	for _, item := range items {
		out.Locations = append(out.Locations, locationView{
			Name: item.Name, Description: item.Description, Country: item.Country,
			City: item.City, NetworkZone: string(item.NetworkZone),
		})
	}
	return nil, out, nil
}

type sshKeysOut struct {
	SSHKeys   []sshKeyView `json:"ssh_keys"`
	Page      pageInfo     `json:"page"`
	RateLimit rateInfo     `json:"rate_limit"`
}

func (a *App) listSSHKeys(ctx context.Context, _ *mcp.CallToolRequest, in namedList) (*mcp.CallToolResult, sshKeysOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, sshKeysOut{}, err
	}
	items, resp, err := client.SSHKey.List(ctx, hcloud.SSHKeyListOpts{ListOpts: in.listOpts(), Name: in.Name})
	if err != nil {
		return nil, sshKeysOut{}, err
	}
	out := sshKeysOut{}
	out.Page, out.RateLimit = metaOf(resp)
	for _, item := range items {
		out.SSHKeys = append(out.SSHKeys, viewSSHKey(item))
	}
	if out.SSHKeys == nil {
		out.SSHKeys = []sshKeyView{}
	}
	return nil, out, nil
}

type createSSHIn struct {
	Name      string            `json:"name"`
	PublicKey string            `json:"public_key" jsonschema:"OpenSSH public key, one line."`
	Labels    map[string]string `json:"labels,omitempty"`
}

func (a *App) createSSHKey(ctx context.Context, _ *mcp.CallToolRequest, in createSSHIn) (*mcp.CallToolResult, sshKeyView, error) {
	client, err := a.api()
	if err != nil {
		return nil, sshKeyView{}, err
	}
	result, _, err := client.SSHKey.Create(ctx, hcloud.SSHKeyCreateOpts{Name: in.Name, PublicKey: in.PublicKey, Labels: in.Labels})
	if err != nil {
		return nil, sshKeyView{}, err
	}
	return nil, viewSSHKey(result), nil
}

func (a *App) deleteSSHKey(ctx context.Context, _ *mcp.CallToolRequest, in deleteIn) (*mcp.CallToolResult, deletedOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, deletedOut{}, err
	}
	key, err := lookupSSHKey(ctx, client, in.ID, in.ConfirmName)
	if err != nil {
		return nil, deletedOut{}, err
	}
	if key.Name != in.ConfirmName {
		return nil, deletedOut{}, fmt.Errorf("confirm_name %q does not match ssh key %q", in.ConfirmName, key.Name)
	}
	if _, err = client.SSHKey.Delete(ctx, key); err != nil {
		return nil, deletedOut{}, err
	}
	return nil, deletedOut{Deleted: true, ID: key.ID, Name: key.Name}, nil
}

type updateSSHKeyIn struct {
	ID   int64  `json:"id" jsonschema:"SSH key id."`
	Name string `json:"name,omitempty" jsonschema:"New key name. Omit to keep the current one."`
	labelPatch
}

func (a *App) updateSSHKey(ctx context.Context, _ *mcp.CallToolRequest, in updateSSHKeyIn) (*mcp.CallToolResult, sshKeyView, error) {
	client, err := a.api()
	if err != nil {
		return nil, sshKeyView{}, err
	}
	if in.Name == "" && in.labelPatch.empty() {
		return nil, sshKeyView{}, fmt.Errorf("pass a name, a label change, or both")
	}
	key, err := lookupSSHKey(ctx, client, in.ID, "")
	if err != nil {
		return nil, sshKeyView{}, err
	}
	labels, changeLabels, err := in.labelPatch.apply(key.Labels)
	if err != nil {
		return nil, sshKeyView{}, err
	}
	opts := hcloud.SSHKeyUpdateOpts{Name: in.Name}
	if changeLabels {
		opts.Labels = labels
	}
	updated, _, err := client.SSHKey.Update(ctx, key, opts)
	if err != nil {
		return nil, sshKeyView{}, err
	}
	return nil, viewSSHKey(updated), nil
}

// lookupSSHKey resolves by id, or by name when the caller only has the name.
func lookupSSHKey(ctx context.Context, client *hcloud.Client, id int64, name string) (*hcloud.SSHKey, error) {
	if id != 0 {
		key, _, err := client.SSHKey.GetByID(ctx, id)
		if err != nil || key == nil {
			return nil, fmt.Errorf("ssh key %d: %w", id, errOrMissing(err))
		}
		return key, nil
	}
	if name != "" {
		key, _, err := client.SSHKey.GetByName(ctx, name)
		if err != nil || key == nil {
			return nil, fmt.Errorf("ssh key %q: %w", name, errOrMissing(err))
		}
		return key, nil
	}
	return nil, fmt.Errorf("pass id or confirm_name")
}

type volumesOut struct {
	Volumes   []volumeView `json:"volumes"`
	Page      pageInfo     `json:"page"`
	RateLimit rateInfo     `json:"rate_limit"`
}

func (a *App) listVolumes(ctx context.Context, _ *mcp.CallToolRequest, in namedList) (*mcp.CallToolResult, volumesOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, volumesOut{}, err
	}
	items, resp, err := client.Volume.List(ctx, hcloud.VolumeListOpts{ListOpts: in.listOpts(), Name: in.Name})
	if err != nil {
		return nil, volumesOut{}, err
	}
	out := volumesOut{}
	out.Page, out.RateLimit = metaOf(resp)
	for _, item := range items {
		out.Volumes = append(out.Volumes, viewVolume(item))
	}
	if out.Volumes == nil {
		out.Volumes = []volumeView{}
	}
	return nil, out, nil
}

type createVolumeIn struct {
	Name      string            `json:"name"`
	Size      int               `json:"size" jsonschema:"Size in GB. Minimum 10. Can only grow later."`
	Location  string            `json:"location,omitempty" jsonschema:"Required unless you also pass server_id."`
	ServerID  int64             `json:"server_id,omitempty" jsonschema:"Attach to this server while creating."`
	Format    string            `json:"format,omitempty" jsonschema:"ext4 or xfs. Optional."`
	Automount *bool             `json:"automount,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Wait      *bool             `json:"wait,omitempty"`
}

type volumeOut struct {
	Volume volumeView  `json:"volume"`
	Action *actionView `json:"action,omitempty"`
}

func (a *App) createVolume(ctx context.Context, _ *mcp.CallToolRequest, in createVolumeIn) (*mcp.CallToolResult, volumeOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, volumeOut{}, err
	}
	if in.Location == "" && in.ServerID == 0 {
		return nil, volumeOut{}, fmt.Errorf("pass location or server_id: Hetzner needs one to place the volume")
	}
	if in.Location != "" && in.ServerID != 0 {
		return nil, volumeOut{}, fmt.Errorf("pass loc" + "ation or server_id, not both: a server already fixes the volume location")
	}
	if in.Size < 10 {
		return nil, volumeOut{}, fmt.Errorf("size must be at least 10 GB")
	}
	if in.Format != "" && in.Format != "ext4" && in.Format != "xfs" {
		return nil, volumeOut{}, fmt.Errorf("format must be ext4 or xfs")
	}
	opts := hcloud.VolumeCreateOpts{Name: in.Name, Size: in.Size, Labels: in.Labels, Automount: in.Automount}
	if in.Format != "" {
		opts.Format = &in.Format
	}
	if in.Location != "" {
		location, _, locErr := client.Location.Get(ctx, in.Location)
		if locErr != nil || location == nil {
			return nil, volumeOut{}, fmt.Errorf("location %q: %w", in.Location, errOrMissing(locErr))
		}
		opts.Location = location
	}
	if in.ServerID != 0 {
		server, lookErr := a.lookupServer(ctx, client, in.ServerID, "")
		if lookErr != nil {
			return nil, volumeOut{}, lookErr
		}
		opts.Server = server
	}
	result, _, err := client.Volume.Create(ctx, opts)
	if err != nil {
		return nil, volumeOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, result.Action); err != nil {
			return nil, volumeOut{}, err
		}
	}
	return nil, volumeOut{Volume: viewVolume(result.Volume), Action: viewAction(result.Action)}, nil
}

type volumeRef struct {
	ID   int64  `json:"id,omitempty" jsonschema:"Volume id. Use this or name."`
	Name string `json:"name,omitempty" jsonschema:"Volume name. Use this or id."`
}

func (a *App) getVolume(ctx context.Context, _ *mcp.CallToolRequest, in volumeRef) (*mcp.CallToolResult, volumeOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, volumeOut{}, err
	}
	volume, err := lookupVolume(ctx, client, in.ID, in.Name)
	if err != nil {
		return nil, volumeOut{}, err
	}
	return nil, volumeOut{Volume: viewVolume(volume)}, nil
}

type updateVolumeIn struct {
	ID   int64  `json:"id" jsonschema:"Volume id."`
	Name string `json:"name,omitempty" jsonschema:"New volume name. Omit to keep the current one."`
	labelPatch
}

func (a *App) updateVolume(ctx context.Context, _ *mcp.CallToolRequest, in updateVolumeIn) (*mcp.CallToolResult, volumeOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, volumeOut{}, err
	}
	if in.Name == "" && in.labelPatch.empty() {
		return nil, volumeOut{}, fmt.Errorf("pass a name, a label change, or both")
	}
	volume, err := lookupVolume(ctx, client, in.ID, "")
	if err != nil {
		return nil, volumeOut{}, err
	}
	labels, changeLabels, err := in.labelPatch.apply(volume.Labels)
	if err != nil {
		return nil, volumeOut{}, err
	}
	opts := hcloud.VolumeUpdateOpts{Name: in.Name}
	if changeLabels {
		opts.Labels = labels
	}
	updated, _, err := client.Volume.Update(ctx, volume, opts)
	if err != nil {
		return nil, volumeOut{}, err
	}
	return nil, volumeOut{Volume: viewVolume(updated)}, nil
}

func lookupVolume(ctx context.Context, client *hcloud.Client, id int64, name string) (*hcloud.Volume, error) {
	switch {
	case id != 0:
		volume, _, err := client.Volume.GetByID(ctx, id)
		if err != nil || volume == nil {
			return nil, fmt.Errorf("volume %d: %w", id, errOrMissing(err))
		}
		return volume, nil
	case name != "":
		volume, _, err := client.Volume.GetByName(ctx, name)
		if err != nil || volume == nil {
			return nil, fmt.Errorf("volume %q: %w", name, errOrMissing(err))
		}
		return volume, nil
	default:
		return nil, fmt.Errorf("pass id or name")
	}
}

type volumeActionIn struct {
	ID          int64  `json:"id"`
	Action      string `json:"action" jsonschema:"attach, detach, resize, or delete."`
	ServerID    int64  `json:"server_id,omitempty" jsonschema:"Required for attach."`
	Size        int    `json:"size,omitempty" jsonschema:"New size in GB for resize. Must be larger."`
	Automount   *bool  `json:"automount,omitempty"`
	ConfirmName string `json:"confirm_name,omitempty" jsonschema:"Required for delete. Must match the volume name."`
	Wait        *bool  `json:"wait,omitempty"`
}

func (a *App) volumeAction(ctx context.Context, _ *mcp.CallToolRequest, in volumeActionIn) (*mcp.CallToolResult, volumeOut, error) {
	client, err := a.api()
	if err != nil {
		return nil, volumeOut{}, err
	}
	volume, err := lookupVolume(ctx, client, in.ID, "")
	if err != nil {
		return nil, volumeOut{}, err
	}
	var action *hcloud.Action
	switch in.Action {
	case "attach":
		if in.ServerID == 0 {
			return nil, volumeOut{}, fmt.Errorf("server_id is required to attach")
		}
		server, lookErr := a.lookupServer(ctx, client, in.ServerID, "")
		if lookErr != nil {
			return nil, volumeOut{}, lookErr
		}
		action, _, err = client.Volume.AttachWithOpts(ctx, volume, hcloud.VolumeAttachOpts{Server: server, Automount: in.Automount})
	case "detach":
		action, _, err = client.Volume.Detach(ctx, volume)
	case "resize":
		if in.Size <= volume.Size {
			return nil, volumeOut{}, fmt.Errorf("size %d GB must be larger than the current %d GB: a volume cannot shrink", in.Size, volume.Size)
		}
		action, _, err = client.Volume.Resize(ctx, volume, in.Size)
	case "delete":
		if volume.Name != in.ConfirmName {
			return nil, volumeOut{}, fmt.Errorf("confirm_name %q does not match volume %q", in.ConfirmName, volume.Name)
		}
		if err = refuseProtected("volume", volume.Name, volume.Protection.Delete); err != nil {
			return nil, volumeOut{}, err
		}
		if _, err = client.Volume.Delete(ctx, volume); err != nil {
			return nil, volumeOut{}, err
		}
		return nil, volumeOut{Volume: viewVolume(volume)}, nil
	default:
		return nil, volumeOut{}, fmt.Errorf("action must be attach, detach, resize, or delete")
	}
	if err != nil {
		return nil, volumeOut{}, err
	}
	if wantWait(in.Wait) {
		if err = a.wait(ctx, action); err != nil {
			return nil, volumeOut{}, err
		}
	}
	if fresh, _, getErr := client.Volume.GetByID(ctx, volume.ID); getErr == nil && fresh != nil {
		volume = fresh
	}
	return nil, volumeOut{Volume: viewVolume(volume), Action: viewAction(action)}, nil
}
