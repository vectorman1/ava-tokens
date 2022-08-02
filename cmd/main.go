/*
 * Copyright 2022 LimeChain Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	tokensvm "ava-tokens/tokens-vm"
	"fmt"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
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

	rpcchainvm.Serve(&tokensvm.VM{})
}
