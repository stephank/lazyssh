// Implements the 'virtualbox' target type, which uses the VirtualBox CLI to
// start/stop existing virtual machines.
package virtualbox

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"

	"github.com/stephank/lazyssh/providers"
)

func init() {
	providers.Register("virtualbox", &Factory{})
}

type Factory struct{}

type Provider struct {
	Name      string
	Addr      string
	CheckPort uint16
	StartMode string
	StopMode  string
	Linger    time.Duration
}

type hclTarget struct {
	Name      string `hcl:"name,attr"`
	Addr      string `hcl:"addr,attr"`
	CheckPort uint16 `hcl:"check_port,optional"`
	StartMode string `hcl:"start_mode,optional"`
	StopMode  string `hcl:"stop_mode,optional"`
	Linger    string `hcl:"linger,optional"`
}

func (factory *Factory) NewProvider(target string, hclBlock hcl.Body) (providers.Provider, error) {
	parsed := &hclTarget{}
	diags := gohcl.DecodeBody(hclBlock, nil, parsed)
	if diags.HasErrors() {
		return nil, diags
	}

	prov := &Provider{
		Name: parsed.Name,
		Addr: parsed.Addr,
	}

	if parsed.CheckPort == 0 {
		prov.CheckPort = 22
	} else {
		prov.CheckPort = parsed.CheckPort
	}

	switch parsed.StartMode {
	case "gui", "headless", "separate":
		prov.StartMode = parsed.StartMode
	case "":
		prov.StartMode = "headless"
	default:
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid start_mode",
			Detail:   fmt.Sprintf("Value '%s' is invalid for start_mode. Must be one of: gui, headless, separate", prov.StartMode),
		})
	}

	switch parsed.StopMode {
	case "poweroff", "acpipowerbutton", "acpisleepbutton":
		prov.StopMode = parsed.StopMode
	case "":
		prov.StopMode = "acpipowerbutton"
	default:
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid stop_mode",
			Detail:   fmt.Sprintf("Value '%s' is invalid for stop_mode. Must be one of: poweroff, acpipowerbutton, acpisleepbutton", prov.StopMode),
		})
	}

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

	return prov, diags
}

func (prov *Provider) IsShared() bool {
	// Shared, because we launch existing virtual machines by name.
	return true
}

func (prov *Provider) RunMachine(mach *providers.Machine) {
	if prov.start() {
		if prov.connectivityTest() {
			prov.msgLoop(mach)
		}
		prov.stop()
	}
}

func (prov *Provider) start() bool {
	// TODO: What to do when the machine is already running?
	cmd := exec.Command("VBoxManage", "startvm", prov.Name, fmt.Sprintf("--type=%s", prov.StartMode))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("VirtualBox machine '%s' failed to start: %s\n", prov.Name, err.Error())
		return false
	}
	log.Printf("Started VirtualBox machine '%s'\n", prov.Name)
	return true
}

func (prov *Provider) stop() {
	cmd := exec.Command("VBoxManage", "controlvm", prov.Name, prov.StopMode)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("VirtualBox machine '%s' failed to stop: %s\n", prov.Name, err.Error())
	}
	log.Printf("Stopped VirtualBox machine '%s'\n", prov.Name)
}

// Check port every 3 seconds for 2 minutes.
func (prov *Provider) connectivityTest() bool {
	checkAddr := fmt.Sprintf("%s:%d", prov.Addr, prov.CheckPort)
	checkTimeout := 3 * time.Second
	var err error
	var conn net.Conn
	for i := 0; i < 40; i++ {
		checkStart := time.Now()
		conn, err = net.DialTimeout("tcp", checkAddr, checkTimeout)
		if err == nil {
			conn.Close()
			log.Printf("Connectivity test succeeded for VirtualBox machine '%s'\n", prov.Name)
			return true
		}
		time.Sleep(time.Until(checkStart.Add(checkTimeout)))
	}
	log.Printf("VirtualBox machine '%s' connectivity test failed: %s\n", prov.Name, err.Error())
	return false
}

func (prov *Provider) msgLoop(mach *providers.Machine) {
	// TODO: Monitor machine status
	active := <-mach.ModActive
	for active > 0 {
		for active > 0 {
			select {
			case mod := <-mach.ModActive:
				active += mod
			case msg := <-mach.Translate:
				msg.Reply <- fmt.Sprintf("%s:%d", prov.Addr, msg.Port)
			case <-mach.Stop:
				return
			}
		}

		// Linger
		select {
		case mod := <-mach.ModActive:
			active += mod
		case <-time.After(time.Duration(prov.Linger) * time.Second):
			return
		}
	}
}
