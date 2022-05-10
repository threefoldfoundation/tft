package bridge

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"

	tfeth "github.com/threefoldfoundation/tft/bsc/bridges/stellar/api"
	"github.com/threefoldfoundation/tft/bsc/bridges/stellar/api/bridge/contract"
	"github.com/threefoldfoundation/tft/bsc/bridges/stellar/api/bridge/mscontract"
)

const ERC20AddressLength = 20

type ERC20Address [ERC20AddressLength]byte

var errNotOwner = errors.New("Bridge is not owner of the multisig contract")

const (
	// retryDelay is the delay to retry calls when there are no peers
	retryDelay = time.Second * 15
)

// BridgeContract exposes a higher lvl api for specific contract bindings. In case of proxy contracts,
// the bridge needs to use the bindings of the implementation contract, but the address of the proxy.
type BridgeContract struct {
	networkConfig tfeth.NetworkConfiguration // Ethereum network
	networkName   string

	lc *LightClient

	tftContract      *Contract
	multisigContract *MsContract

	// cache some stats in case they might be usefull
	head    *types.Header // Current head header of the bridge
	balance *big.Int      // The current balance of the bridge (note: ethers only!)
	nonce   uint64        // Current pending nonce of the bridge
	price   *big.Int      // Current gas price to issue funds with

	lock sync.RWMutex // Lock protecting the bridge's internals
}

type Contract struct {
	filter     *contract.TokenFilterer
	transactor *contract.TokenTransactor
	caller     *contract.TokenCaller

	contract *bind.BoundContract
	abi      abi.ABI
}

type MsContract struct {
	filter     *mscontract.TokenFilterer
	transactor *mscontract.TokenTransactor
	caller     *mscontract.TokenCaller

	contract *bind.BoundContract
	abi      abi.ABI
}

// GetContractAdress returns the address of this contract
func (bridge *BridgeContract) GetContractAdress() common.Address {
	return bridge.networkConfig.ContractAddress
}

// NewBridgeContract creates a new wrapper for an allready deployed contract
func NewBridgeContract(bridgeConfig *BridgeConfig) (*BridgeContract, error) {
	fmt.Printf("bridge is Follower %v", bridgeConfig.Follower)
	// load correct network config
	networkConfig, err := tfeth.GetEthNetworkConfiguration(bridgeConfig.EthNetworkName)
	if err != nil {
		return nil, err
	}
	// override contract address if it's provided
	if bridgeConfig.ContractAddress != "" {
		log.Info("Overriding default token contract", "address", bridgeConfig.ContractAddress)
		networkConfig.ContractAddress = common.HexToAddress(bridgeConfig.ContractAddress)
		// TODO: validate ABI of contract,
		//       see https://github.com/threefoldtech/rivine-extension-erc20/issues/3
	}
	// override contract address if it's provided
	if bridgeConfig.MultisigContractAddress != "" {
		log.Info("Overriding default multisig contract", "address", bridgeConfig.MultisigContractAddress)
		networkConfig.MultisigContractAddress = common.HexToAddress(bridgeConfig.MultisigContractAddress)
		// TODO: validate ABI of contract,
		//       see https://github.com/threefoldtech/rivine-extension-erc20/issues/3
	}

	bootstrapNodes, err := networkConfig.GetBootnodes()
	if err != nil {
		return nil, err
	}
	staticNodes, err := networkConfig.GetStaticNodes()
	if err != nil {
		return nil, err
	}
	lc, err := NewLightClient(LightClientConfig{
		Port:           bridgeConfig.EthPort,
		DataDir:        bridgeConfig.Datadir,
		BootstrapNodes: bootstrapNodes,
		StaticNodes:    staticNodes,
		NetworkName:    networkConfig.NetworkName,
		NetworkID:      networkConfig.NetworkID,
		GenesisBlock:   networkConfig.GenesisBlock,
	})
	if err != nil {
		return nil, err
	}
	err = lc.LoadAccount(bridgeConfig.AccountJSON, bridgeConfig.AccountPass)
	if err != nil {
		return nil, err
	}

	tftContract, err := createTft20Contract(networkConfig, lc.Client)
	if err != nil {
		return nil, err
	}

	multisigContract, err := createMultisigContract(networkConfig, lc.Client)
	if err != nil {
		return nil, err
	}

	return &BridgeContract{
		networkName:      bridgeConfig.EthNetworkName,
		networkConfig:    networkConfig,
		lc:               lc,
		tftContract:      tftContract,
		multisigContract: multisigContract,
	}, nil
}

func createTft20Contract(networkConfig tfeth.NetworkConfiguration, client *ethclient.Client) (*Contract, error) {
	log.Info("Creating token contract binding", "address", networkConfig.ContractAddress)
	filter, err := contract.NewTokenFilterer(networkConfig.ContractAddress, client)
	if err != nil {
		return nil, err
	}

	transactor, err := contract.NewTokenTransactor(networkConfig.ContractAddress, client)
	if err != nil {
		return nil, err
	}

	caller, err := contract.NewTokenCaller(networkConfig.ContractAddress, client)
	if err != nil {
		return nil, err
	}

	tft20contract, tft20abi, err := bindTTFT20(networkConfig.ContractAddress, client, client, client)
	if err != nil {
		return nil, err
	}

	return &Contract{
		filter:     filter,
		transactor: transactor,
		caller:     caller,
		contract:   tft20contract,
		abi:        tft20abi,
	}, nil
}

func createMultisigContract(networkConfig tfeth.NetworkConfiguration, client *ethclient.Client) (*MsContract, error) {
	log.Info("Creating multisig contract binding", "address", networkConfig.MultisigContractAddress)
	filter, err := mscontract.NewTokenFilterer(networkConfig.MultisigContractAddress, client)
	if err != nil {
		return nil, err
	}

	transactor, err := mscontract.NewTokenTransactor(networkConfig.MultisigContractAddress, client)
	if err != nil {
		return nil, err
	}

	caller, err := mscontract.NewTokenCaller(networkConfig.MultisigContractAddress, client)
	if err != nil {
		return nil, err
	}

	multisigContract, multisigAbi, err := bindMultisig(networkConfig.MultisigContractAddress, client, client, client)
	if err != nil {
		return nil, err
	}

	return &MsContract{
		filter:     filter,
		transactor: transactor,
		caller:     caller,
		contract:   multisigContract,
		abi:        multisigAbi,
	}, nil
}

// Close terminates the Ethereum connection and tears down the stack.
func (bridge *BridgeContract) Close() error {
	return bridge.lc.Close()
}

// AccountAddress returns the account address of the bridge contract
func (bridge *BridgeContract) AccountAddress() (common.Address, error) {
	return bridge.lc.AccountAddress()
}

// LightClient returns the LightClient driving this bridge contract
func (bridge *BridgeContract) LightClient() *LightClient {
	return bridge.lc
}

// Refresh attempts to retrieve the latest header from the chain and extract the
// associated bridge balance and nonce for connectivity caching.
func (bridge *BridgeContract) Refresh(head *types.Header) error {
	// Ensure a state update does not run for too long
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// If no header was specified, use the current chain head
	var err error
	if head == nil {
		if head, err = bridge.lc.HeaderByNumber(ctx, nil); err != nil {
			return err
		}
	}
	// Retrieve the balance, nonce and gas price from the current head
	var (
		nonce   uint64
		price   *big.Int
		balance *big.Int
	)
	if price, err = bridge.lc.SuggestGasPrice(ctx); err != nil {
		return err
	}
	log.Info("Suggested gas price is now", "price", price)
	if balance, err = bridge.lc.AccountBalanceAt(ctx, head.Number); err != nil {
		return err
	}
	log.Debug(bridge.lc.account.account.Address.Hex())
	// Everything succeeded, update the cached stats
	bridge.lock.Lock()
	bridge.head, bridge.balance = head, balance
	bridge.price, bridge.nonce = price, nonce
	bridge.lock.Unlock()
	return nil
}

// Loop subscribes to new eth heads. If a new head is received, it is passed on the given channel,
// after which the internal stats are updated if no update is already in progress
func (bridge *BridgeContract) Loop(ch chan<- *types.Header) {
	log.Debug("Subscribing to eth headers")
	// channel to receive head updates from client on
	heads := make(chan *types.Header, 16)
	// subscribe to head upates
	sub, err := bridge.lc.SubscribeNewHead(context.Background(), heads)
	if err != nil {
		log.Error("Failed to subscribe to head events", "err", err)
	}
	defer sub.Unsubscribe()
	// channel so we can update the internal state from the heads
	// update := make(chan *types.Header)
	// go func() {
	// 	for head := range update {
	// 		// old heads should be ignored during a chain sync after some downtime
	// 		if err := bridge.Refresh(head); err != nil {
	// 			log.Warn("Failed to update state", "block", head.Number, "err", err)
	// 		}
	// 		log.Debug("Internal stats updated", "block", head.Number, "account balance", bridge.balance, "gas price", bridge.price, "nonce", bridge.nonce)
	// 	}
	// }()
	for head := range heads {
		ch <- head
		select {
		// only process new head if another isn't being processed yet
		// case update <- head:
		// 	log.Debug("Processing new head")
		default:
			log.Debug("Ignoring current head, update already in progress")
		}
	}
	log.Error("Bridge state update loop ended")
}

// SubscribeTransfers subscribes to new Transfer events on the given contract. This call blocks
// and prints out info about any transfer as it happened
func (bridge *BridgeContract) SubscribeTransfers() error {
	sink := make(chan *contract.TokenTransfer)
	opts := &bind.WatchOpts{Context: context.Background(), Start: nil}
	sub, err := bridge.tftContract.filter.WatchTransfer(opts, sink, nil, nil)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	for {
		select {
		case err = <-sub.Err():
			return err
		case transfer := <-sink:
			log.Debug("Noticed transfer event", "from", transfer.From, "to", transfer.To, "amount", transfer.Tokens)
		}
	}
}

// SubscribeMint subscribes to new Mint events on the given contract. This call blocks
// and prints out info about any mint as it happened
func (bridge *BridgeContract) SubscribeMint() error {
	sink := make(chan *contract.TokenMint)
	opts := &bind.WatchOpts{Context: context.Background(), Start: nil}
	sub, err := bridge.tftContract.filter.WatchMint(opts, sink, nil, nil)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	for {
		select {
		case err = <-sub.Err():
			return err
		case mint := <-sink:
			log.Info("Noticed mint event", "receiver", mint.Receiver, "amount", mint.Tokens, "TFT tx id", mint.Txid)
		}
	}
}

// SubmissionEvent holds relevant information about a submission event
type SubmissionEvent struct {
	transactionId *big.Int
}

// Receiver of the withdraw
func (c SubmissionEvent) TransactionId() *big.Int {
	return c.transactionId
}

// SubscribeSubmission subscribes to new submission event on the given multisig contract.
func (bridge *BridgeContract) SubscribeSubmission(submitChan chan<- SubmissionEvent) error {
	sink := make(chan *mscontract.TokenSubmission)
	opts := &bind.WatchOpts{Context: context.Background(), Start: nil}
	sub, err := bridge.multisigContract.filter.WatchSubmission(opts, sink, nil)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	for {
		select {
		case err = <-sub.Err():
			return err
		case submission := <-sink:
			log.Info("Noticed submissions event", "txid", submission.TransactionId)
			submitChan <- SubmissionEvent{
				transactionId: submission.TransactionId,
			}
		}
	}
}

// WithdrawEvent holds relevant information about a withdraw event
type WithdrawEvent struct {
	receiver           common.Address
	amount             *big.Int
	blockchain_address string
	network            string
	txHash             common.Hash
	blockHash          common.Hash
	blockHeight        uint64
	raw                []byte
}

// Receiver of the withdraw
func (w WithdrawEvent) Receiver() common.Address {
	return w.receiver
}

// Amount withdrawn
func (w WithdrawEvent) Amount() *big.Int {
	return w.amount
}

// Blockchain address to withdraw to
func (w WithdrawEvent) BlockchainAddress() string {
	return w.blockchain_address
}

// Network to withdraw to
func (w WithdrawEvent) Network() string {
	return w.network
}

// TxHash hash of the transaction
func (w WithdrawEvent) TxHash() common.Hash {
	return w.txHash
}

// BlockHash of the containing block
func (w WithdrawEvent) BlockHash() common.Hash {
	return w.blockHash
}

// BlockHeight of the containing block
func (w WithdrawEvent) BlockHeight() uint64 {
	return w.blockHeight
}

// SubscribeWithdraw subscribes to new Withdraw events on the given contract. This call blocks
// and prints out info about any withdraw as it happened
func (bridge *BridgeContract) SubscribeWithdraw(wc chan<- WithdrawEvent, startHeight uint64) error {
	log.Info("Subscribing to withdraw events", "start height", startHeight)
	sink := make(chan *contract.TokenWithdraw)
	watchOpts := &bind.WatchOpts{Context: context.Background(), Start: nil}
	sub, err := bridge.WatchWithdraw(watchOpts, sink, nil)
	if err != nil {
		log.Error("Subscribing to withdraw events failed", "err", err)
		return err
	}
	defer sub.Unsubscribe()
	for {
		select {
		case err = <-sub.Err():
			return err
		case withdraw := <-sink:
			if withdraw.Raw.Removed {
				// ignore removed events
				continue
			}
			log.Debug("Noticed withdraw event", "receiver", withdraw.Receiver, "amount", withdraw.Tokens)
			wc <- WithdrawEvent{
				receiver:           withdraw.Receiver,
				amount:             withdraw.Tokens,
				txHash:             withdraw.Raw.TxHash,
				blockHash:          withdraw.Raw.BlockHash,
				blockHeight:        withdraw.Raw.BlockNumber,
				blockchain_address: withdraw.BlockchainAddress,
				network:            withdraw.Network,
				raw:                withdraw.Raw.Data,
			}
		}
	}
}

// WatchWithdraw is a free log subscription operation binding the contract event 0x884edad9ce6fa2440d8a54cc123490eb96d2768479d49ff9c7366125a9424364.
//
// Solidity: e Withdraw(receiver indexed address, tokens uint256)
//
// This method is copied from the generated bindings and slightly modified, so we can add logic to stay backwards compatible with the old withdraw event signature
func (bridge *BridgeContract) WatchWithdraw(opts *bind.WatchOpts, sink chan<- *contract.TokenWithdraw, receiver []common.Address) (event.Subscription, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := bridge.tftContract.contract.WatchLogs(opts, "Withdraw", receiverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(contract.TokenWithdraw)
				if err := bridge.tftContract.contract.UnpackLog(event, "Withdraw", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// FilterWithdraw filters Withdraw events on the given contract. This call blocks
// and prints out info about any withdraw as it happened
func (bridge *BridgeContract) FilterWithdraw(wc chan<- WithdrawEvent, startHeight uint64, endHeight uint64) error {
	log.Info("Filtering to withdraw events", "start height", startHeight, "end height", endHeight)
	filterOpts := bind.FilterOpts{
		Start: startHeight,
		End:   &endHeight,
	}
	withdrawEvent, err := bridge.tftContract.filter.FilterWithdraw(&filterOpts, nil)
	if err != nil {
		log.Error("filtering withdraw events failed", "err", err)
		return err
	}

	for withdrawEvent.Next() {
		if withdrawEvent.Event == nil {
			break
		}

		log.Info("Withdraw event found", "event", withdrawEvent)
		wc <- WithdrawEvent{
			receiver:           withdrawEvent.Event.Receiver,
			amount:             withdrawEvent.Event.Tokens,
			txHash:             withdrawEvent.Event.Raw.TxHash,
			blockHash:          withdrawEvent.Event.Raw.BlockHash,
			blockHeight:        withdrawEvent.Event.Raw.BlockNumber,
			blockchain_address: withdrawEvent.Event.BlockchainAddress,
			network:            withdrawEvent.Event.Network,
			raw:                withdrawEvent.Event.Raw.Data,
		}
	}
	return nil
}

// TransferFunds transfers funds from one address to another
func (bridge *BridgeContract) TransferFunds(recipient common.Address, amount *big.Int) error {
	err := bridge.transferFunds(recipient, amount)
	for IsNoPeerErr(err) {
		log.Warn("no peers while trying to transfer funds, retrying...")
		time.Sleep(retryDelay)
		err = bridge.transferFunds(recipient, amount)
	}
	return err
}

func (bridge *BridgeContract) transferFunds(recipient common.Address, amount *big.Int) error {
	if amount == nil {
		return errors.New("invalid amount")
	}
	accountAddress, err := bridge.lc.AccountAddress()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	opts := &bind.TransactOpts{
		Context: ctx, From: accountAddress,
		Signer: bridge.getSignerFunc(),
		Value:  nil, Nonce: nil, GasLimit: 0, GasPrice: nil,
	}
	_, err = bridge.tftContract.transactor.Transfer(opts, recipient, amount)
	return err
}

func (bridge *BridgeContract) Mint(receiver ERC20Address, amount *big.Int, txID string) error {
	err := bridge.mint(receiver, amount, txID)
	for IsNoPeerErr(err) {
		log.Warn("no peers while trying to mint, retrying...")
		time.Sleep(retryDelay)
		err = bridge.mint(receiver, amount, txID)
	}
	return err
}

func (bridge *BridgeContract) mint(receiver ERC20Address, amount *big.Int, txID string) error {
	log.Info("Calling mint function in contract")
	if amount == nil {
		return errors.New("invalid amount")
	}

	owners, err := bridge.multisigContract.caller.GetOwners(&bind.CallOpts{})
	if err != nil {
		return err
	}

	accountAddress, err := bridge.lc.AccountAddress()
	if err != nil {
		return err
	}

	ownerExists := false
	for _, owner := range owners {
		if owner == accountAddress {
			ownerExists = true
			break
		}
	}

	if !ownerExists {
		for _, owner := range owners {
			log.Info("Multisig contract owner", "owner", owner)
		}
		log.Error("Not an owner of the multisig contract", "account", accountAddress)
		return errNotOwner
	}

	bytes, err := bridge.tftContract.abi.Pack("mintTokens", common.Address(receiver), amount, txID)
	log.Info("Calling mint function")
	if err != nil {
		return err
	}

	gas, err := bridge.lc.SuggestGasPrice(context.Background())
	if err != nil {
		return err
	}
	newGas := big.NewInt(10 * gas.Int64())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	opts := &bind.TransactOpts{
		Context: ctx, From: accountAddress,
		Signer: bridge.getSignerFunc(),
		Value:  nil, Nonce: nil, GasLimit: 1000000, GasPrice: newGas,
	}

	log.Info("Submitting transaction to multisig contract", "tokenaddress", bridge.networkConfig.ContractAddress)
	_, err = bridge.multisigContract.transactor.SubmitTransaction(opts, common.Address(bridge.networkConfig.ContractAddress), big.NewInt(0), bytes)
	if err != nil {
		return err
	}

	return nil
}

func (bridge *BridgeContract) ConfirmTransaction(txid *big.Int) error {
	isConfirmed, err := bridge.IsConfirmedTxID(txid)
	if err != nil {
		return err
	}

	if isConfirmed {
		return nil
	}

	log.Info("Going to confirm transaction")
	accountAddress, err := bridge.lc.AccountAddress()
	if err != nil {
		return err
	}

	// TODO estimate gas more correctly ..
	gas, err := bridge.lc.SuggestGasPrice(context.Background())
	if err != nil {
		return err
	}
	newGas := big.NewInt(10 * gas.Int64())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	opts := &bind.TransactOpts{
		Context: ctx, From: accountAddress,
		Signer: bridge.getSignerFunc(),
		Value:  nil, Nonce: nil, GasLimit: 1000000, GasPrice: newGas,
	}

	log.Info("Confirming transaction on multisig contract")
	_, err = bridge.multisigContract.transactor.ConfirmTransaction(opts, txid)
	return err
}

func (bridge *BridgeContract) IsConfirmedTxID(txID *big.Int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	opts := &bind.CallOpts{Context: ctx}
	return bridge.multisigContract.caller.IsConfirmed(opts, txID)
}

func (bridge *BridgeContract) GetTransactionByID(txID *big.Int) (struct {
	Destination common.Address
	Value       *big.Int
	Data        []byte
	Executed    bool
}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	opts := &bind.CallOpts{Context: ctx}
	return bridge.multisigContract.caller.Transactions(opts, txID)
}

func (bridge *BridgeContract) IsMintTxID(txID string) (bool, error) {
	res, err := bridge.isMintTxID(txID)
	for IsNoPeerErr(err) {
		log.Warn("no peers while trying to check mint txid, retrying...")
		time.Sleep(retryDelay)
		res, err = bridge.isMintTxID(txID)
	}
	return res, err
}

func (bridge *BridgeContract) isMintTxID(txID string) (bool, error) {
	log.Debug("Calling isMintID")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	opts := &bind.CallOpts{Context: ctx}
	return bridge.tftContract.caller.IsMintID(opts, txID)
}

func (bridge *BridgeContract) getSignerFunc() bind.SignerFn {
	return func(signer types.Signer, address common.Address, tx *types.Transaction) (*types.Transaction, error) {
		accountAddress, err := bridge.lc.AccountAddress()
		if err != nil {
			return nil, err
		}
		if address != accountAddress {
			return nil, errors.New("not authorized to sign this account")
		}
		networkID := int64(bridge.networkConfig.NetworkID)
		return bridge.lc.SignTx(tx, big.NewInt(networkID))
	}
}

func (bridge *BridgeContract) TokenBalance(address common.Address) (*big.Int, error) {
	log.Debug("Calling TokenBalance function in contract")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	opts := &bind.CallOpts{Context: ctx}
	return bridge.tftContract.caller.BalanceOf(opts, common.Address(address))
}

func (bridge *BridgeContract) EthBalance() (*big.Int, error) {
	err := bridge.Refresh(nil) // force a refresh
	return bridge.balance, err
}

// bindTTFT20 binds a generic wrapper to an already deployed contract.
//
// This method is copied from the generated bindings as a convenient way to get a *bind.Contract, as this is needed to implement the WatchWithdraw function ourselves
func bindTTFT20(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, abi.ABI, error) {
	parsed, err := abi.JSON(strings.NewReader(contract.TokenABI))
	if err != nil {
		return nil, parsed, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), parsed, nil
}

// bindMultisig binds a generic wrapper to an already deployed contract.
//
// This method is copied from the generated bindings as a convenient way to get a *bind.Contract, as this is needed to implement the WatchWithdraw function ourselves
func bindMultisig(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, abi.ABI, error) {
	parsed, err := abi.JSON(strings.NewReader(mscontract.TokenABI))
	if err != nil {
		return nil, parsed, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), parsed, nil
}
