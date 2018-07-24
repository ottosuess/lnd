package lntest

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/rpcclient"
)

var (
	harnessNetParams = &chaincfg.RegressionNetParams
)

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

func NewBtcdBackend() (*BtcdBackendConfig, func(), error) {
	// Now create a new rpctest.harness that will act as our backend node.
	// Add an argument to ensure we are connecting the chain backend to the
	// miner.
	args := []string{"--rejectnonstd", "--debuglevel=debug"}
	chainBackend, err := rpctest.New(harnessNetParams, nil, args)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create btcd node: %v", err)
	}

	// We must set up the chain backend first, such that the wallet will be
	// able to sync up to the chain created by the miner when it is set up.
	if err := chainBackend.SetUp(false, 0); err != nil {
		return nil, nil, fmt.Errorf("unable to set up btcd backend: %v", err)
	}

	if err := chainBackend.Node.NotifyNewTransactions(false); err != nil {
		return nil, nil, fmt.Errorf("unable to request transaction notifications: %v", err)
	}

	bd := &BtcdBackendConfig{
		RPCConfig: chainBackend.RPCConfig(),
		Harness:   chainBackend,
	}

	cleanUp := func() {
		//	defer chainBackend.TearDown()
	}

	return bd, cleanUp, nil
}
