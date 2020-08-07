package manager

import (
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/stephank/lazyssh/providers"
	"golang.org/x/crypto/ssh"
)

// channelOpenDirectMsg is used to unmarshal the payload of the SSH
// direct-tcpip channel open request. (RFC 4254 7.2)
type channelOpenDirectMsg struct {
	RemoteAddr string
	RemotePort uint32
	LocalAddr  string
	LocalPort  uint32
}

// machine is a Machine wrapper with internal Manager fields added.
type machine struct {
	providers.Machine

	// target is the virtual address of the target this machine belongs to.
	target string
	// shared indicates whether IsShared was true at the time the machine was
	// created. If true, the machine will be in sharedMachines.
	shared bool
}

// machines is an index of running machines.
type machines map[*machine]struct{}

// sharedMachines is an index of shared running machines by target address.
type sharedMachines map[string]*machine

// Manager is the central piece responsible for starting/stopping machines
// using a Provider, and connecting SSH channels to the actual TCP port onn the
// target machine.
//
// Ownership of the Manager struct lies in the goroutine started in NewManager.
// Public methods on the Manager provide an interface to communicate with the
// goroutine. (This is essentially the agent pattern.)
type Manager struct {
	newChannel  chan ssh.NewChannel
	stop        chan chan struct{}
	machStopped chan *machine
	providers   providers.Providers
	machines
	sharedMachines
}

// NewManager creates a new Manager from the given Providers, and starts the
// main goroutine running the Manager message loop.
//
// Ownership of the Providers passed in is transferred to the Manager.
// Specifically, Provider methods are called from the Manager goroutine.
func NewManager(provs providers.Providers) *Manager {
	mgr := &Manager{
		newChannel:     make(chan ssh.NewChannel),
		stop:           make(chan chan struct{}),
		machStopped:    make(chan *machine),
		providers:      provs,
		machines:       make(machines),
		sharedMachines: make(sharedMachines),
	}
	go func() {
		var stoppingCh []chan struct{}
		for stoppingCh == nil || len(mgr.machines) > 0 {
			select {
			case newChan := <-mgr.newChannel:
				if stoppingCh == nil {
					mgr.handleNewChannel(newChan)
				} else {
					newChan.Reject(ssh.Prohibited, "this server is shutting down")
				}
			case mach := <-mgr.machStopped:
				mgr.handleMachineStopped(mach)
			case replyCh := <-mgr.stop:
				if stoppingCh == nil {
					for mach := range mgr.machines {
						mach.Stop <- struct{}{}
					}
				}
				stoppingCh = append(stoppingCh, replyCh)
			}
		}
		for _, ch := range stoppingCh {
			ch <- struct{}{}
		}
	}()
	return mgr
}

// NewChannel transfers an SSH channel to the Manager for processing.
//
// The Manager will verify the channel is 'direct-tcpip' channel and parse
// parameters, start the target machine if necessary, then connect the channel
// to the requested TCP port on the target machine.
func (mgr *Manager) NewChannel(newChan ssh.NewChannel) {
	mgr.newChannel <- newChan
}

// Stop instructs the Manager to shutdown.
//
// Once the Manager goroutine receives the stop message, it will shut down all
// machines and reject any further requests. The Stop method waits for all
// machines to shut down before returning.
func (mgr *Manager) Stop() {
	replyCh := make(chan struct{})
	mgr.stop <- replyCh
	<-replyCh
}

// handleNewChannel processes an SSH channel sent to the Manager.
//
// Runs on the Manager message loop goroutine. A separate goroutine is launched
// for the Provider to do processing on.
func (mgr *Manager) handleNewChannel(newChan ssh.NewChannel) {
	if newChan.ChannelType() != "direct-tcpip" {
		newChan.Reject(ssh.UnknownChannelType, "unsuported channel type")
		return
	}

	input := channelOpenDirectMsg{}
	if err := ssh.Unmarshal(newChan.ExtraData(), &input); err != nil {
		newChan.Reject(ssh.Prohibited, "invalid direct-tcpip parameters")
		return
	}

	prov, ok := mgr.providers[input.RemoteAddr]
	if !ok {
		newChan.Reject(ssh.ConnectionFailed, "unknown remote address")
		return
	}

	// Try for a shared machine, otherwise start a new one.
	var mach *machine
	if prov.IsShared() {
		mach = mgr.sharedMachines[input.RemoteAddr]
	}

	if mach == nil {
		mach = &machine{
			target: input.RemoteAddr,
			Machine: providers.Machine{
				ModActive: make(chan int8),
				Translate: make(chan *providers.TranslateMsg),
				Stop:      make(chan struct{}, 1),
			},
		}

		log.Printf("Starting machine for target '%s'\n", mach.target)
		go func() {
			prov.RunMachine(&mach.Machine)
			mgr.machStopped <- mach
		}()

		mgr.machines[mach] = struct{}{}
		if prov.IsShared() {
			mach.shared = true
			mgr.sharedMachines[mach.target] = mach
		}
	}

	// Further connection setup is async, don't block the Manager message loop.
	go connectChannel(newChan, mach, input)
}

// connectChannel connects an SSH channel to a TCP port on a machine.
//
// Runs on a dedicated goroutine per channel, so is free to block.
func connectChannel(newChan ssh.NewChannel, mach *machine, input channelOpenDirectMsg) {
	// Inform the Provider about active connections.
	incActive(mach)
	defer decActive(mach)

	// Request translation of the SSH direct-tcpip input parameters to a Dialer
	// address. Providers do not respond to this until the machine is ready, so
	// we'll block here.
	msg := &providers.TranslateMsg{
		Addr:  input.RemoteAddr,
		Port:  uint16(input.RemotePort),
		Reply: make(chan string),
	}
	mach.Translate <- msg
	addr := <-msg.Reply
	if addr == "" {
		// Usually happens when a request arrives during machine shutdown, but the
		// Provider may also send this as an abort instruction for whatever reason.
		newChan.Reject(ssh.ConnectionFailed, "service not available")
		return
	}

	// Connect and drive I/O in separate goroutines.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		newChan.Reject(ssh.ConnectionFailed, err.Error())
		return
	}

	tcp := conn.(*net.TCPConn)
	ch, reqs, err := newChan.Accept()
	if err != nil {
		tcp.Close()
		return
	}

	defer ch.Close()
	go ssh.DiscardRequests(reqs)
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer tcp.CloseWrite()
		io.Copy(tcp, ch)
	}()

	go func() {
		defer wg.Done()
		defer tcp.CloseRead()
		defer ch.CloseWrite()
		io.Copy(ch, tcp)
	}()

	// The WaitGroup ensures defers wait until I/O in *both* directions ends.
	wg.Wait()
}

// handleMachineStopped takes care of cleanup after a Machine stops.
//
// Runs on the Manager message loop goroutine. When the Provider RunMachine
// method ends, a message is sent to the Manager, which brings us here.
func (mgr *Manager) handleMachineStopped(mach *machine) {
	log.Printf("Stopped machine for target '%s'\n", mach.target)
	delete(mgr.machines, mach)
	if mach.shared {
		delete(mgr.sharedMachines, mach.target)
	}

	// Discard any connectChannel messages that may have raced us here. 5 seconds
	// should be ample, because this should only have the cover the time between
	// the above deletes and any in-progress connectChannel goroutine startup.
	go func() {
		for {
			select {
			case <-mach.ModActive:
				continue
			case msg := <-mach.Translate:
				msg.Reply <- ""
			case <-time.After(5 * time.Second):
				return
			}
		}
	}()
}

func incActive(mach *machine) {
	mach.ModActive <- +1
}

func decActive(mach *machine) {
	mach.ModActive <- -1
}
