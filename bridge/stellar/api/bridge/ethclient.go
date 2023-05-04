package bridge

import (
	"context"
	"errors"
	"math/big"
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/light"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"

	tfeth "github.com/threefoldfoundation/tft/bridge/stellar/eth"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// LightClient creates a light client that can be used to interact with the Ethereum network,
// for ERC20 purposes. By default it is read-only, in order to also write to the network,
// you'll need load an account using the LoadAccount method.
type LightClient struct {
	*ethclient.Client // Client connection to the Ethereum chain
	// optional account info
	datadir     string
	accountLock sync.RWMutex
	account     *clientAccountInfo
}

type clientAccountInfo struct {
	keystore *keystore.KeyStore // Keystore containing the signing info
	account  accounts.Account   // Account funding the bridge requests
}

// LightClientConfig combines all configuration required for
// creating and configuring a LightClient.
type LightClientConfig struct {
	DataDir      string
	NetworkName  string
	EthUrl       string
	NetworkID    uint64
	GenesisBlock *core.Genesis
}

func (lccfg *LightClientConfig) validate() error {
	if lccfg.DataDir == "" {
		return errors.New("invalid LightClientConfig: no data directory defined")
	}
	if lccfg.NetworkName == "" {
		return errors.New("invalid LightClientConfig: no network name defined")
	}
	if lccfg.EthUrl == "" {
		return errors.New("invalid LightClientConfig: no network url defined")
	}
	if lccfg.NetworkID == 0 {
		return errors.New("invalid LightClientConfig: no network ID defined")
	}
	return nil
}

// NewLightClient creates a new light client that can be used to interact with the ETH network.
// See `LightClient` for more information.
func NewLightClient(lccfg LightClientConfig) (*LightClient, error) {
	// validate the cfg, as to provide better error reporting for obvious errors
	err := lccfg.validate()
	if err != nil {
		return nil, err
	}

	cl, err := ethclient.Dial(lccfg.EthUrl)
	if err != nil {
		return nil, err
	}
	datadir := filepath.Join(lccfg.DataDir, lccfg.NetworkName)
	// return created light client
	return &LightClient{
		Client:  cl,
		datadir: datadir,
	}, nil
}

// LoadAccount loads an account into this light client,
// allowing writeable operations using the loaded account.
// An error is returned in case no account could be loaded.
func (lc *LightClient) LoadAccount(accountJSON, accountPass string) error {
	// create keystore
	ks, err := tfeth.InitializeKeystore(lc.datadir, accountJSON, accountPass)
	if err != nil {
		return err
	}
	lc.accountLock.Lock()
	lc.account = &clientAccountInfo{
		keystore: ks,
		account:  ks.Accounts()[0],
	}
	lc.accountLock.Unlock()
	return nil
}

var (
	// ErrNoAccountLoaded is an error returned for all Light Client methods
	// that require an account and for which no account is loaded.
	ErrNoAccountLoaded = errors.New("no account was loaded into the light client")
)

// AccountBalanceAt returns the balance for the account at the given block height.
func (lc *LightClient) AccountBalanceAt(ctx context.Context, blockNumber *big.Int) (*big.Int, error) {
	lc.accountLock.RLock()
	defer lc.accountLock.RUnlock()
	if lc.account == nil {
		return nil, ErrNoAccountLoaded
	}
	return lc.BalanceAt(ctx, lc.account.account.Address, blockNumber)
}

// SignTx signs a given traction with the loaded account, returning the signed transaction and no error on success.
func (lc *LightClient) SignTx(tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	lc.accountLock.RLock()
	defer lc.accountLock.RUnlock()
	if lc.account == nil {
		return nil, ErrNoAccountLoaded
	}
	return lc.account.keystore.SignTx(lc.account.account, tx, chainID)
}

// AccountAddress returns the address of the loaded account,
// returning an error only if no account was loaded.
func (lc *LightClient) AccountAddress() (common.Address, error) {
	lc.accountLock.RLock()
	defer lc.accountLock.RUnlock()
	var addr common.Address
	if lc.account == nil {
		return addr, ErrNoAccountLoaded
	}
	copy(addr[:], lc.account.account.Address[:])
	return addr, nil
}

// IsNoPeerErr checks if an error is means an ethereum client could not execute
// a call because it has no valid peers
func IsNoPeerErr(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == light.ErrNoPeers.Error()
}

// GetBalanceInfo returns bridge ethereum address and balance
func (lc *LightClient) GetBalanceInfo() (*ERC20BalanceInfo, error) {
	lc.accountLock.RLock()
	defer lc.accountLock.RUnlock()
	var addr common.Address

	if lc.account == nil {
		return nil, ErrNoAccountLoaded
	}
	copy(addr[:], lc.account.account.Address[:])

	balance, err := lc.BalanceAt(context.Background(), addr, nil)

	if err != nil {
		return nil, err
	}

	return &ERC20BalanceInfo{
		Balance: balance,
		Address: lc.account.account.Address,
	}, nil
}

// ERC20BalanceInfo provides a definition for the ethereum bridge address balance
type ERC20BalanceInfo struct {
	Balance *big.Int       `json:"balance"`
	Address common.Address `json:"address"`
}
