package lntest

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/rpcclient"
)

// BtcdBackendConfig is an implementation of the BackendConfig interface
// backed by a btcd node.
type BtcdBackendConfig struct {
	RPCConfig rpcclient.ConnConfig
	Harness   *rpctest.Harness
}

func (b BtcdBackendConfig) GenArgs() []string {
	var args []string
	encodedCert := hex.EncodeToString(b.RPCConfig.Certificates)
	args = append(args, "--bitcoin.node=btcd")
	args = append(args, fmt.Sprintf("--btcd.rpchost=%v", b.RPCConfig.Host))
	args = append(args, fmt.Sprintf("--btcd.rpcuser=%v", b.RPCConfig.User))
	args = append(args, fmt.Sprintf("--btcd.rpcpass=%v", b.RPCConfig.Pass))
	args = append(args, fmt.Sprintf("--btcd.rawrpccert=%v", encodedCert))

	return args
}

func (b BtcdBackendConfig) P2PAddr() string {
	return b.Harness.P2PAddress()
}

// NewBtcdBackend starts a new rpctest.Harness and returns a BtcdBackendConfig
// for that node.
func NewBtcdBackend() (*BtcdBackendConfig, func(), error) {
	args := []string{"--rejectnonstd"}
	netParams := &chaincfg.RegressionNetParams
	chainBackend, err := rpctest.New(netParams, nil, args)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create btcd node: %v", err)
	}

	if err := chainBackend.SetUp(false, 0); err != nil {
		return nil, nil, fmt.Errorf("unable to set up btcd backend: %v", err)
	}

	bd := &BtcdBackendConfig{
		RPCConfig: chainBackend.RPCConfig(),
		Harness:   chainBackend,
	}

	cleanUp := func() {
		defer chainBackend.TearDown()
	}

	return bd, cleanUp, nil
}
