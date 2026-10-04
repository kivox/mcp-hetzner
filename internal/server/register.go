package server

import "github.com/modelcontextprotocol/go-sdk/mcp"

func (a *App) register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "find_operations",
		Description: "Search the Hetzner Cloud API catalog. Use this before cloud_api. An empty query returns tags. The server, not you, will perform the call.",
		Annotations: readOnly("Find Cloud API operations"),
	}, a.findOperations)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "cloud_get",
		Description: "GET one Cloud API path from find_operations. The server authenticates and performs the request. Read-only. Add query page and per_page when a list is incomplete.",
		Annotations: readOnly("GET a Cloud API path"),
	}, a.cloudGet)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "cloud_api",
		Description: "POST, PUT, or DELETE one Cloud API path from find_operations. The server authenticates and performs the request. Use cloud_get for GET. Never include a token or hostname.",
		Annotations: destructive("Change the Cloud API"),
	}, a.cloudAPI)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_action",
		Description: "Read one Hetzner action by id. Use this when a previous tool returned an action and wait was false.",
		Annotations: readOnly("Get action"),
	}, a.getAction)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_servers",
		Description: "List servers in this project, one page at a time. Call again with next_page when it is set.",
		Annotations: readOnly("List servers"),
	}, a.listServers)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_server",
		Description: "Get one server by id or name, including public IPs and status.",
		Annotations: readOnly("Get server"),
	}, a.getServer)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_server",
		Description: "Create a server. The server resolves type, image, location, and SSH keys, then sends the request. Waits by default. Returns a root password only when no SSH key was used.",
		Annotations: additive("Create server"),
	}, a.createServer)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_server",
		Description: "Rename a server or replace its labels. Omitted fields stay as they are. An empty labels object clears every label.",
		Annotations: additive("Update server"),
	}, a.updateServer)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_server_protection",
		Description: "Turn delete or rebuild protection on or off. Disable delete protection before delete_server. The server sends the action and waits by default.",
		Annotations: additive("Set server protection"),
	}, a.setServerProtection)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_server",
		Description: "Delete a server. confirm_name must exactly match the live name from get_server.",
		Annotations: destructive("Delete server"),
	}, a.deleteServer)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "server_power",
		Description: "Change server power. Prefer shutdown. poweroff and reset are hard power cuts. The server sends the action and waits by default.",
		Annotations: destructive("Change server power"),
	}, a.serverPower)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_server_types",
		Description: "List server types with cores, memory, and disk. Use the name as create_server.server_type.",
		Annotations: readOnly("List server types"),
	}, a.listServerTypes)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_images",
		Description: "List images. Defaults to system images so the result stays small. Use the name as create_server.image.",
		Annotations: readOnly("List images"),
	}, a.listImages)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_locations",
		Description: "List datacenter locations. Use the name as create_server.location.",
		Annotations: readOnly("List locations"),
	}, a.listLocations)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_networks",
		Description: "List private networks with their IP range and how many servers are attached. Change a network with cloud_api.",
		Annotations: readOnly("List networks"),
	}, a.listNetworks)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_load_balancers",
		Description: "List load balancers with public IPs and target counts. Change one with cloud_api.",
		Annotations: readOnly("List load balancers"),
	}, a.listLoadBalancers)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_primary_ips",
		Description: "List primary IPs and the resource each is assigned to.",
		Annotations: readOnly("List primary IPs"),
	}, a.listPrimaryIPs)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_floating_ips",
		Description: "List floating IPs and the server each is assigned to.",
		Annotations: readOnly("List floating IPs"),
	}, a.listFloatingIPs)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_ssh_keys",
		Description: "List SSH keys in the project. Pass a key name to create_server.",
		Annotations: readOnly("List SSH keys"),
	}, a.listSSHKeys)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_ssh_key",
		Description: "Store an OpenSSH public key in the project. The server sends it to Hetzner. Do not send a private key.",
		Annotations: additive("Create SSH key"),
	}, a.createSSHKey)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_ssh_key",
		Description: "Delete an SSH key. confirm_name must match the key name.",
		Annotations: destructive("Delete SSH key"),
	}, a.deleteSSHKey)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_volumes",
		Description: "List volumes, including which server they are attached to.",
		Annotations: readOnly("List volumes"),
	}, a.listVolumes)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_volume",
		Description: "Create a volume. Size is in GB and can only grow later. The server sends the request and waits by default.",
		Annotations: additive("Create volume"),
	}, a.createVolume)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "volume_action",
		Description: "Attach, detach, resize, or delete a volume. Delete requires confirm_name. The server performs the action.",
		Annotations: destructive("Change volume"),
	}, a.volumeAction)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_firewalls",
		Description: "List firewalls and their rules.",
		Annotations: readOnly("List firewalls"),
	}, a.listFirewalls)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_firewall",
		Description: "Get one firewall by id or name, including rules.",
		Annotations: readOnly("Get firewall"),
	}, a.getFirewall)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_firewall",
		Description: "Create a firewall and its rules. Apply it afterwards with apply_firewall.",
		Annotations: additive("Create firewall"),
	}, a.createFirewall)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_firewall_rules",
		Description: "Replace all rules on a firewall. The server sends the full rule list. An empty list clears the firewall.",
		Annotations: destructive("Set firewall rules"),
	}, a.setFirewallRules)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "apply_firewall",
		Description: "Apply or remove a firewall on a server or a label selector. Set remove to true to detach it.",
		Annotations: additive("Apply firewall"),
	}, a.applyFirewall)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_firewall",
		Description: "Delete a firewall. confirm_name must match the firewall name.",
		Annotations: destructive("Delete firewall"),
	}, a.deleteFirewall)
}
