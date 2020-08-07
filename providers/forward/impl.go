// Implements the 'forward' type, which is essentially a dummy that doesn't
// really make any external calls, but simply forwards connections to a fixed
// address.
package forward

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/stephank/lazyssh/providers"
)

type Factory struct{}

type Provider struct {
	To string
}

type hclTarget struct {
	To string `hcl:"to,attr"`
}

func (factory *Factory) NewProvider(target string, hclBlock hcl.Body) (providers.Provider, error) {
	parsed := &hclTarget{}
	if diags := gohcl.DecodeBody(hclBlock, nil, parsed); diags != nil {
		return nil, diags
	}

	prov := &Provider{
		To: parsed.To,
	}

	return prov, nil
}

func (prov *Provider) IsShared() bool {
	return true
}

func (prov *Provider) RunMachine(mach *providers.Machine) {
	// Once started, we just never stop the shared Machine. This means we waste a
	// goroutine per 'forward' target, but that's negligible.
	for {
		select {
		case <-mach.ModActive:
			continue
		case msg := <-mach.Translate:
			msg.Reply <- fmt.Sprintf("%s:%d", prov.To, msg.Port)
		case <-mach.Stop:
			return
		}
	}
}
