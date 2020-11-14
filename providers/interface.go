/*
Package providers defines the interfaces that LazySSH provider implementations
must conform to.
*/
package providers

import (
	"sync"

	"github.com/hashicorp/hcl/v2"
)

var (
	factoryMapMu sync.Mutex
	FactoryMap   Factories
)

// Register registers a provider factory.
func Register(id string, f Factory) {
	factoryMapMu.Lock()
	defer factoryMapMu.Unlock()
	if FactoryMap == nil {
		FactoryMap = make(map[string]Factory)
	}
	if _, dup := FactoryMap[id]; dup {
		panic("Register called twice for provider " + id)
	}
	FactoryMap[id] = f
}

// Factory produces a Provider for a specific type of Machine, based on
// 'target' configuration provided by the user.
type Factory interface {
	NewProvider(target string, hclBlock hcl.Body) (Provider, error)
}

// Factories is an index of Factory objects by Machine type name.
type Factories map[string]Factory

// Provider is responsible for managing the Machine lifecycle.
//
// Structs implementing Provider encapsulate parsed and validated 'target'
// configuration, and are created by the associated Factory for the type of
// Machine requested.
//
// Note that methods on Provider may be called from different goroutines. See
// the documentation of each method for details.
type Provider interface {
	// IsShared indicates if multiple connections to the same target will not
	// start a new Machine if one is already running.
	//
	// Called from the Manager message loop goroutine, and should not block.
	IsShared() bool

	// RunMachine is responsible for managing the entire Machine lifecycle.
	//
	// Runs on a dedicated goroutine per machine, so is free to block.
	//
	// Typically, the Provider will immediately make external calls to start the
	// machine, wait for proper connectivity to the Machine, then go into a loop
	// processing the messages on the various channels on the Machine struct.
	//
	// Once the Provider determines there is no more activity via ModActive
	// messages, or when it receives a Stop message, it exits the message loop
	// and makes the necessary external calls to stop the machine again.
	// Specifically, this method should not return without stopping the machine.
	RunMachine(mach *Machine)
}

// Providers is an index of configured Provider instances by Machine type name.
type Providers map[string]Provider

// Machine represents a running machine, and holds channels via which the
// Provider receives commands from the Manager.
type Machine struct {
	// ModActive messages indicate activity on the Machine. A message +1
	// indicates a new forwarded TCP connection is opened, and a message -1
	// indicates a TCP connection was closed.
	ModActive chan int8
	// Translate messages are requests to translate SSH direct-tcpip parameters
	// to a Dialer address. The provider should not process/reply to these
	// messages until it has verified connectivity to the Machine.
	Translate chan *TranslateMsg
	// Stop messages are sent by the Manager to request the Machine immediately
	// shut down.
	Stop chan struct{}
	// State can be used by the provider to store machine-specific state.
	State interface{}
}

// TranslateMsg is the type sent on the Machine Translate channel.
type TranslateMsg struct {
	// Addr is the address the SSH client wants to connect to. It contains user
	// input verbatim, so may be a IP address, hostname, etc.
	Addr string
	// Port is the TCP port the SSH client wants to connect to.
	Port uint16
	// Reply is the channel the translation result is sent to. The result is a
	// Dailer address used to make the actual TCP connection to the Machine. The
	// provider should not send a reply until it has verified connectivity to the
	// Machine.
	Reply chan string
}
