package main

import (
	"crypto/sha256"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stephank/lazyssh/providers"
	"golang.org/x/crypto/ssh"
)

// hclFiles is a File index expected by the DiagnosticWriter.
type hclFiles map[string]*hcl.File

// hclConfig is used to unmarshal the HCL top-level.
type hclConfig struct {
	Server  hclServerConfig   `hcl:"server,block"`
	Targets []hclTargetConfig `hcl:"target,block"`
}

// hclServerConfig is used to unmarshal the HCL `server` block.
type hclServerConfig struct {
	Listen        string `hcl:"listen,optional"`
	HostKey       string `hcl:"host_key,attr"`
	AuthorizedKey string `hcl:"authorized_key,attr"`
}

// hclTargetConfig is used to unmarshal HCL `target` blocks.
type hclTargetConfig struct {
	Addr     string `hcl:"addr,label"`
	Type     string `hcl:"type,label"`
	hcl.Body `hcl:"body,remain"`
}

// config is the result of parsing and validation the HCL configuration.
type config struct {
	Listen        string
	HostKey       ssh.Signer
	AuthorizedKey [32]byte
	providers.Providers
}

// Parse a file containing HCL configuration.
//
// This method returns a hclFiles used in printing diagnostics, the *config
// which is non-nil on success, and Diagnostics which may be non-nil on even
// when successful.
func parseConfigFile(cfgFile string, factories providers.Factories) (hclFiles, *config, hcl.Diagnostics) {
	// Step one: basic HCL parsing, without schema.
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile(cfgFile)
	files := parser.Files()
	if diags.HasErrors() {
		// Can't provide more info if this doesn't succeed.
		return files, nil, diags
	}

	// Step two: Partial unmarshal using hclConfig and implied schema.
	// Specifically, this does not unmarshal 'target' blocks.
	hclConfig := hclConfig{}
	if diags = gohcl.DecodeBody(file.Body, nil, &hclConfig); diags.HasErrors() {
		// Can't provide more info if this doesn't succeed.
		return files, nil, diags
	}

	// Step three: Defaults and further field parsing.
	//
	// If these fail, we add diagnostics but continue to provide more feedback.
	if hclConfig.Server.Listen == "" {
		hclConfig.Server.Listen = "localhost:7922"
	}

	hostKey, err := ssh.ParsePrivateKey([]byte(hclConfig.Server.HostKey))
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Could not parse server host_key",
			Detail:   err.Error(),
		})
	}

	authorizedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hclConfig.Server.AuthorizedKey))
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Could not parse server authorized_key",
			Detail:   err.Error(),
		})
	}

	// Step four: For each 'target', ask the Factory for the associated type to
	// parse config and instantiate a Provider.
	//
	// If these fail, we add diagnostics but continue to provide more feedback.
	providers := make(providers.Providers)
	for _, hclTarget := range hclConfig.Targets {
		_, exists := providers[hclTarget.Addr]
		if exists {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Duplicate target address",
				Detail:   fmt.Sprintf("Each target must have a unique address, but '%s' was used in multiple target definitions", hclTarget.Addr),
			})
		}

		factory, ok := factories[hclTarget.Type]
		if !ok {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid provider type",
				Detail:   fmt.Sprintf("Target '%s' has invalid provider type '%s'", hclTarget.Addr, hclTarget.Type),
			})
			continue
		}

		prov, err := factory.NewProvider(hclTarget.Addr, hclTarget.Body)
		provDiags, ok := err.(hcl.Diagnostics)
		if !ok && err != nil {
			provDiags = hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Provider configuration error",
					Detail:   fmt.Sprintf("Error in '%s' provider configuration for target '%s': %s", hclTarget.Type, hclTarget.Addr, err.Error()),
				},
			}
		}

		diags = append(diags, provDiags...)
		if !provDiags.HasErrors() {
			providers[hclTarget.Addr] = prov
		}
	}

	// Make sure we return nil Config if there are any errors.
	if diags.HasErrors() {
		return files, nil, diags
	}

	cfg := &config{
		Listen:        hclConfig.Server.Listen,
		HostKey:       hostKey,
		AuthorizedKey: sha256.Sum256(authorizedKey.Marshal()),
		Providers:     providers,
	}
	return files, cfg, diags
}
