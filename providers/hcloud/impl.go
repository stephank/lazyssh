// Implements the 'hcloud' target type, which uses HCLOUD SDK to create
// and terminate hcloud virtual machines.
package hcloud

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"golang.org/x/net/context"

	"github.com/stephank/lazyssh/providers"
)

func init() {
	providers.Register("hcloud", &Factory{})
}

type Factory struct{}

type Provider struct {
	Image      string
	ServerType string
	SSHKey     string
	UserData   string
	Location   string
	Shared     bool
	CheckPort  uint16
	Linger     time.Duration
	HCloud     *hcloud.Client
}

type state struct {
	id   string
	addr *string
}

type hclTarget struct {
	Token      string `hcl:"token,attr"`
	Image      string `hcl:"image,attr"`
	ServerType string `hcl:"server_type,attr"`
	SSHKey     string `hcl:"ssh_key,attr"`
	Location   string `hcl:"location,attr"`
	UserData   string `hcl:"user_data,optional"`
	CheckPort  uint16 `hcl:"check_port,optional"`
	Shared     *bool  `hcl:"shared,optional"`
	Linger     string `hcl:"linger,optional"`
}

const requestTimeout = 30 * time.Second

func (factory *Factory) NewProvider(target string, hclBlock hcl.Body) (providers.Provider, error) {
	parsed := &hclTarget{}
	diags := gohcl.DecodeBody(hclBlock, nil, parsed)
	if diags.HasErrors() {
		return nil, diags
	}

	client := hcloud.NewClient(
		hcloud.WithApplication("lazyssh", ""),
		hcloud.WithToken(parsed.Token),
	)

	prov := &Provider{
		HCloud:     client,
		Image:      parsed.Image,
		ServerType: parsed.ServerType,
		SSHKey:     parsed.SSHKey,
		Location:   parsed.Location,
		UserData:   strings.Replace(parsed.UserData, "\n", "\\n", -1),
	}

	if parsed.CheckPort == 0 {
		prov.CheckPort = 22
	} else {
		prov.CheckPort = parsed.CheckPort
	}

	if parsed.Shared == nil {
		prov.Shared = true
	} else {
		prov.Shared = *parsed.Shared
	}

	if prov.Shared {
		linger, err := time.ParseDuration(parsed.Linger)
		if err == nil {
			prov.Linger = linger
		} else {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid duration for 'linger' field",
				Detail:   fmt.Sprintf("The 'linger' value '%s' is not a valid duration: %s", parsed.Linger, err.Error()),
			})
		}
	} else if parsed.Linger != "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagWarning,
			Summary:  "Field 'linger' was ignored",
			Detail:   fmt.Sprintf("The 'linger' field has no effect for 'hcloud' targets with 'shared = false'"),
		})
	}

	if diags.HasErrors() {
		return nil, diags
	}

	return prov, diags
}

func (prov *Provider) IsShared() bool {
	return prov.Shared
}

func (prov *Provider) RunMachine(mach *providers.Machine) {
	if prov.start(mach) {
		if prov.connectivityTest(mach) {
			prov.msgLoop(mach)
		}
		prov.stop(mach)
	}
}

func (prov *Provider) start(mach *providers.Machine) bool {
	bgCtx := context.Background()

	// We must get the image from API
	ctx, _ := context.WithTimeout(bgCtx, requestTimeout)
	image, _, err := prov.HCloud.Image.Get(ctx, prov.Image)
	if err != nil {
		log.Printf("HCloud server failed to start: %s\n", err.Error())
		return false
	}
	// We must get the server type from API
	ctx, _ = context.WithTimeout(bgCtx, requestTimeout)
	serverType, _, err := prov.HCloud.ServerType.Get(ctx, prov.ServerType)
	if err != nil {
		log.Printf("HCloud server failed to start: %s\n", err.Error())
		return false
	}
	// We must get the SSH key from API
	ctx, _ = context.WithTimeout(bgCtx, requestTimeout)
	sshKey, _, err := prov.HCloud.SSHKey.Get(ctx, prov.SSHKey)
	if err != nil {
		log.Printf("HCloud server failed to start: %s\n", err.Error())
		return false
	}
	// We must get the Location from API
	ctx, _ = context.WithTimeout(bgCtx, requestTimeout)
	location, _, err := prov.HCloud.Location.Get(ctx, prov.Location)
	if err != nil {
		log.Printf("HCloud server failed to start: %s\n", err.Error())
		return false
	}

	opts := hcloud.ServerCreateOpts{
		Name:             "lazyssh",
		ServerType:       serverType,
		Image:            image,
		SSHKeys:          []*hcloud.SSHKey{sshKey},
		Location:         location,
		UserData:         prov.UserData,
		StartAfterCreate: hcloud.Bool(true),
	}

	res, _, err := prov.HCloud.Server.Create(ctx, opts)
	if err != nil {
		log.Printf("HCloud server failed to start: %s\n", err.Error())
		return false
	}

	server := res.Server
	log.Printf("Created HCloud server '%s'\n", server.Name)

	for i := 0; i < 20 && serverIsStarting(server); i++ {
		<-time.After(3 * time.Second)

		ctx, _ := context.WithTimeout(bgCtx, requestTimeout)
		res, _, err := prov.HCloud.Server.GetByID(ctx, server.ID)
		if err != nil {
			log.Printf("Could not check HCloud server '%s' state: %s\n", server.Name, err.Error())
			return false
		}

		server = res
	}

	if server.Status != hcloud.ServerStatusRunning {
		log.Printf("HCloud server '%s' in unexpected state '%s'\n", server.Name, server.Status)
		return false
	}

	log.Printf("HCloud server '%s' is running\n", server.Name)

	address := server.PublicNet.IPv4.IP.String()
	mach.State = &state{
		id:   server.Name,
		addr: &address,
	}
	return true
}

func serverIsStarting(server *hcloud.Server) bool {
	return server.Status == hcloud.ServerStatusInitializing ||
		server.Status == hcloud.ServerStatusStarting ||
		server.Status == hcloud.ServerStatusOff
}

func (prov *Provider) stop(mach *providers.Machine) {
	state := mach.State.(*state)
	bgCtx := context.Background()
	ctx, _ := context.WithTimeout(bgCtx, requestTimeout)
	server, _, err := prov.HCloud.Server.GetByName(ctx, state.id)
	if err != nil {
		log.Printf("HCloud server '%s' not found: %s\n", state.id, err.Error())
		return
	}
	ctx, _ = context.WithTimeout(bgCtx, requestTimeout)
	_, err = prov.HCloud.Server.Delete(ctx, server)
	if err != nil {
		log.Printf("HCloud server '%s' failed to stop: %s\n", state.id, err.Error())
	}
	log.Printf("Terminated HCloud server '%s'\n", state.id)
}

// Check port every 3 seconds for 2 minutes.
func (prov *Provider) connectivityTest(mach *providers.Machine) bool {
	state := mach.State.(*state)
	if state.addr == nil {
		log.Printf("HCloud server '%s' does not have a public IP address\n", state.id)
		return false
	}
	checkAddr := fmt.Sprintf("%s:%d", *state.addr, prov.CheckPort)
	checkTimeout := 3 * time.Second
	var err error
	var conn net.Conn
	for i := 0; i < 40; i++ {
		checkStart := time.Now()
		conn, err = net.DialTimeout("tcp", checkAddr, checkTimeout)
		if err == nil {
			conn.Close()
			log.Printf("Connectivity test succeeded for HCloud server '%s'\n", state.id)
			return true
		}
		time.Sleep(time.Until(checkStart.Add(checkTimeout)))
	}
	log.Printf("HCloud server '%s' port check failed: %s\n", state.id, err.Error())
	return false
}

func (prov *Provider) msgLoop(mach *providers.Machine) {
	// TODO: Monitor machine status
	state := mach.State.(*state)
	active := <-mach.ModActive
	for active > 0 {
		for active > 0 {
			select {
			case mod := <-mach.ModActive:
				active += mod
			case msg := <-mach.Translate:
				msg.Reply <- fmt.Sprintf("%s:%d", *state.addr, msg.Port)
			case <-mach.Stop:
				return
			}
		}

		// Linger
		select {
		case mod := <-mach.ModActive:
			active += mod
		case <-time.After(prov.Linger):
			return
		}
	}
}
