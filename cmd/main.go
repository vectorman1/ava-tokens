package main

import (
	tokensvm "ava-tokens/tokens-vm"
	"fmt"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/hashicorp/go-plugin"
	log "github.com/inconshreveable/log15"
	"os"
)

func main() {
	version, err := PrintVersion()
	if err != nil {
		fmt.Printf("couldn't get config: %s", err)
		os.Exit(1)
	}
	// Print VM ID and exit
	if version {
		fmt.Printf("%s@%s\n", tokensvm.Name, tokensvm.Version)
		os.Exit(0)
	}

	log.Root().SetHandler(
		log.LvlFilterHandler(
			log.LvlDebug,
			log.StreamHandler(os.Stderr, log.TerminalFormat()),
		),
	)

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: rpcchainvm.Handshake,
		Plugins: map[string]plugin.Plugin{
			"vm": rpcchainvm.New(&tokensvm.VM{}),
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
