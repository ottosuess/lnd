package lntest

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
)

// BitcoindBackendConfig is an implementation of the BackendConfig interface
// backed by a Bitcoind node.
type BitcoindBackendConfig struct {
	RPCHost string
	RPCUser string
	RPCPass string
	ZMQPath string
	P2PPort int
}

func (b BitcoindBackendConfig) GenArgs() []string {
	var args []string
	args = append(args, "--bitcoin.node=bitcoind")
	args = append(args, fmt.Sprintf("--bitcoind.rpchost=%v", b.RPCHost))
	args = append(args, fmt.Sprintf("--bitcoind.rpcuser=%v", b.RPCUser))
	args = append(args, fmt.Sprintf("--bitcoind.rpcpass=%v", b.RPCPass))
	args = append(args, fmt.Sprintf("--bitcoind.zmqpath=%v", b.ZMQPath))

	return args
}

func (b BitcoindBackendConfig) P2PAddr() string {
	return fmt.Sprintf("127.0.0.1:%v", b.P2PPort)
}

// NewBitocindBackend starts a bitcoind node and returns a BitoindBackendConfig
// for that node.
func NewBitcoindBackend() (*BitcoindBackendConfig, func(), error) {
	tempBitcoindDir, err := ioutil.TempDir("", "bitcoind")
	if err != nil {
		return nil, nil,
			fmt.Errorf("unable to create temp directory: %v", err)
	}

	zmqPath := "ipc:///" + tempBitcoindDir + "/weks.socket"
	rpcPort := rand.Int()%(65536-1024) + 1024
	p2pPort := rand.Int()%(65536-1024) + 1024

	bitcoind := exec.Command(
		"bitcoind",
		"-datadir="+tempBitcoindDir,
		"-regtest",
		"-txindex",
		"-whitelist=127.0.0.1", // whitelist localhost to speed up relay.
		"-rpcauth=weks:469e9bb14ab2360f8e226efed5ca6f"+
			"d$507c670e800a95284294edb5773b05544b"+
			"220110063096c221be9933c82d38e1",
		fmt.Sprintf("-rpcport=%d", rpcPort),
		fmt.Sprintf("-port=%d", p2pPort),
		"-disablewallet",
		"-zmqpubrawblock="+zmqPath,
		"-zmqpubrawtx="+zmqPath,
	)

	err = bitcoind.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't start bitcoind: %v", err)
	}

	bd := BitcoindBackendConfig{
		RPCHost: fmt.Sprintf("localhost:%v", rpcPort),
		RPCUser: "weks",
		RPCPass: "weks",
		ZMQPath: zmqPath,
		P2PPort: p2pPort,
	}

	cleanUp := func() {
		defer os.RemoveAll(tempBitcoindDir)
		defer bitcoind.Wait()
		defer bitcoind.Process.Kill()
	}

	return &bd, cleanUp, nil
}
