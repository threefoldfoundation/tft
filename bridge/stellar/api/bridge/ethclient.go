package bridge

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthClient creates a light client that can be used to interact with the Ethereum network,
type EthClient struct {
	*ethclient.Client // Client connection to the Ethereum chain
	privateKey        *ecdsa.PrivateKey
	address           common.Address
}

// LightClientConfig combines all configuration required for
// creating and configuring a EthClient.
type LightClientConfig struct {
	NetworkName   string
	EthUrl        string
	NetworkID     uint64
	EthPrivateKey string
	GenesisBlock  *core.Genesis
}

func (lccfg *LightClientConfig) validate() error {
	if lccfg.NetworkName == "" {
		return errors.New("invalid LightClientConfig: no network name defined")
	}
	if lccfg.EthUrl == "" {
		return errors.New("invalid LightClientConfig: no network url defined")
	}
	if lccfg.NetworkID == 0 {
		return errors.New("invalid LightClientConfig: no network ID defined")
	}
	if lccfg.EthPrivateKey == "" {
		return errors.New("invalid LightClientConfig: no private key defined")
	}
	return nil
}

// NewLiEthient creates a new light client that can be used to interact with the ETH network.
// See `EthClient` for more information.
func NewEthClient(lccfg LightClientConfig) (*EthClient, error) {
	// validate the cfg, as to provide better error reporting for obvious errors
	err := lccfg.validate()
	if err != nil {
		return nil, err
	}

	log.Debug("private key", "k", strings.Trim(lccfg.EthPrivateKey, "0x"))
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(lccfg.EthPrivateKey, "0x"))
	if err != nil {
		return nil, err
	}
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("error casting public key to ECDSA")
	}

	addr := crypto.PubkeyToAddress(*publicKeyECDSA)

	cl, err := ethclient.Dial(lccfg.EthUrl)
	if err != nil {
		return nil, err
	}
	// return created light client
	return &EthClient{
		Client:     cl,
		privateKey: privateKey,
		address:    addr,
	}, nil
}

func (c *EthClient) GetAddress() (common.Address, error) {
	publicKey := c.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return common.Address{}, errors.New("error casting public key to ECDSA")
	}

	return crypto.PubkeyToAddress(*publicKeyECDSA), nil
}

var (
	// ErrNoAccountLoaded is an error returned for all Light Client methods
	// that require an account and for which no account is loaded.
	ErrNoAccountLoaded = errors.New("no account was loaded into the light client")
)

// AccountBalanceAt returns the balance for the account at the given block height.
func (c *EthClient) AccountBalanceAt(ctx context.Context, blockNumber *big.Int) (*big.Int, error) {
	return c.BalanceAt(ctx, c.address, blockNumber)
}

// SignTx signs a given traction with the loaded account, returning the signed transaction and no error on success.
func (c *EthClient) SignTx(tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	return types.SignTx(tx, types.NewEIP155Signer(chainID), c.privateKey)
}

func (c *EthClient) SignHash(hash common.Hash) ([]byte, error) {
	return crypto.Sign(hash.Bytes(), c.privateKey)
}

// AccountAddress returns the address of the loaded account,
// returning an error only if no account was loaded.
func (c *EthClient) AccountAddress() (common.Address, error) {
	return c.address, nil
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
func (c *EthClient) GetBalanceInfo() (*ERC20BalanceInfo, error) {
	balance, err := c.BalanceAt(context.Background(), c.address, nil)

	if err != nil {
		return nil, err
	}

	return &ERC20BalanceInfo{
		Balance: balance,
		Address: c.address,
	}, nil
}

// ERC20BalanceInfo provides a definition for the ethereum bridge address balance
type ERC20BalanceInfo struct {
	Balance *big.Int       `json:"balance"`
	Address common.Address `json:"address"`
}
