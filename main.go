package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/hcl/v2"
	"github.com/stephank/lazyssh/manager"
	"github.com/stephank/lazyssh/providers"
	_ "github.com/stephank/lazyssh/providers/aws_ec2"
	_ "github.com/stephank/lazyssh/providers/forward"
	_ "github.com/stephank/lazyssh/providers/virtualbox"
	"golang.org/x/crypto/ssh"
)

func main() {
	configFile := flag.String("config", "config.hcl", "config file")
	flag.Parse()

	// Parse config and always print diagnostics, but only fail on errors.
	files, config, diags := parseConfigFile(*configFile, providers.FactoryMap)
	stdoutInfo, _ := os.Stdout.Stat()
	isTty := (stdoutInfo.Mode() & os.ModeCharDevice) != 0
	writer := hcl.NewDiagnosticTextWriter(os.Stdout, files, 80, isTty)
	writer.WriteDiagnostics(diags)
	if diags.HasErrors() {
		os.Exit(1)
	}

	manager := manager.NewManager(config.Providers)

	sshConfig := &ssh.ServerConfig{}
	sshConfig.AddHostKey(config.HostKey)

	sshConfig.PublicKeyCallback = func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if conn.User() != "jump" {
			return nil, errors.New("Unauthorized")
		}

		input := sha256.Sum256(key.Marshal())
		if subtle.ConstantTimeCompare(input[:], config.AuthorizedKey[:]) == 0 {
			return nil, errors.New("Unauthorized")
		}

		return nil, nil
	}

	sshConfig.AuthLogCallback = func(conn ssh.ConnMetadata, method string, err error) {
		if err == nil {
			log.Printf("%v %s auth success\n", conn.RemoteAddr(), method)
		} else {
			log.Printf("%v %s auth attempt: %v\n", conn.RemoteAddr(), method, err)
		}
	}

	listener, err := net.Listen("tcp", config.Listen)
	if err != nil {
		log.Printf("Could not bind to port: %s\n", err)
		os.Exit(1)
	}

	log.Printf("Listening on %s\n", config.Listen)

	exitStatus := 0
	stopping := false
	termCh := make(chan os.Signal)
	signal.Notify(termCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			rawConn, err := listener.Accept()
			if err != nil {
				if stopping {
					break
				}
				exitStatus = 1
				log.Printf("Could not accept connection: %s\n", err.Error())
				termCh <- syscall.SIGTERM
				return
			}

			go func() {
				conn, newChannels, reqs, err := ssh.NewServerConn(rawConn, sshConfig)
				if err != nil {
					log.Printf("%v handshake failed: %s\n", rawConn.RemoteAddr(), err.Error())
					return
				}

				defer conn.Close()
				go ssh.DiscardRequests(reqs)

				for ch := range newChannels {
					manager.NewChannel(ch)
				}
			}()
		}
	}()

	// Only handle one interruption. The next one hard-exits the process.
	<-termCh
	signal.Reset()

	stopping = true
	listener.Close()
	log.Printf("Stopping all machines\n")
	manager.Stop()
	log.Printf("Shutdown complete\n")
	os.Exit(exitStatus)
}
