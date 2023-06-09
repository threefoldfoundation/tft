// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// AccountActivationMetaData contains all meta data concerning the AccountActivation contract.
var AccountActivationMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"network\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"account\",\"type\":\"string\"}],\"name\":\"ActivateAccount\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousBeneficiary\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newBeneficiary\",\"type\":\"address\"}],\"name\":\"BeneficiaryChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"network\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"cost\",\"type\":\"uint256\"}],\"name\":\"NetworkCostChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"network\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"account\",\"type\":\"string\"}],\"name\":\"activateAccount\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"beneficiary\",\"outputs\":[{\"internalType\":\"addresspayable\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"addresspayable\",\"name\":\"newBeneficiary\",\"type\":\"address\"}],\"name\":\"changeBeneficiary\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"network\",\"type\":\"string\"}],\"name\":\"networkCost\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"network\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"cost\",\"type\":\"uint256\"}],\"name\":\"setNetworkCost\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// AccountActivationABI is the input ABI used to generate the binding from.
// Deprecated: Use AccountActivationMetaData.ABI instead.
var AccountActivationABI = AccountActivationMetaData.ABI

// AccountActivation is an auto generated Go binding around an Ethereum contract.
type AccountActivation struct {
	AccountActivationCaller     // Read-only binding to the contract
	AccountActivationTransactor // Write-only binding to the contract
	AccountActivationFilterer   // Log filterer for contract events
}

// AccountActivationCaller is an auto generated read-only Go binding around an Ethereum contract.
type AccountActivationCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AccountActivationTransactor is an auto generated write-only Go binding around an Ethereum contract.
type AccountActivationTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AccountActivationFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type AccountActivationFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AccountActivationSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type AccountActivationSession struct {
	Contract     *AccountActivation // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// AccountActivationCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type AccountActivationCallerSession struct {
	Contract *AccountActivationCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// AccountActivationTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type AccountActivationTransactorSession struct {
	Contract     *AccountActivationTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// AccountActivationRaw is an auto generated low-level Go binding around an Ethereum contract.
type AccountActivationRaw struct {
	Contract *AccountActivation // Generic contract binding to access the raw methods on
}

// AccountActivationCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type AccountActivationCallerRaw struct {
	Contract *AccountActivationCaller // Generic read-only contract binding to access the raw methods on
}

// AccountActivationTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type AccountActivationTransactorRaw struct {
	Contract *AccountActivationTransactor // Generic write-only contract binding to access the raw methods on
}

// NewAccountActivation creates a new instance of AccountActivation, bound to a specific deployed contract.
func NewAccountActivation(address common.Address, backend bind.ContractBackend) (*AccountActivation, error) {
	contract, err := bindAccountActivation(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &AccountActivation{AccountActivationCaller: AccountActivationCaller{contract: contract}, AccountActivationTransactor: AccountActivationTransactor{contract: contract}, AccountActivationFilterer: AccountActivationFilterer{contract: contract}}, nil
}

// NewAccountActivationCaller creates a new read-only instance of AccountActivation, bound to a specific deployed contract.
func NewAccountActivationCaller(address common.Address, caller bind.ContractCaller) (*AccountActivationCaller, error) {
	contract, err := bindAccountActivation(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &AccountActivationCaller{contract: contract}, nil
}

// NewAccountActivationTransactor creates a new write-only instance of AccountActivation, bound to a specific deployed contract.
func NewAccountActivationTransactor(address common.Address, transactor bind.ContractTransactor) (*AccountActivationTransactor, error) {
	contract, err := bindAccountActivation(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &AccountActivationTransactor{contract: contract}, nil
}

// NewAccountActivationFilterer creates a new log filterer instance of AccountActivation, bound to a specific deployed contract.
func NewAccountActivationFilterer(address common.Address, filterer bind.ContractFilterer) (*AccountActivationFilterer, error) {
	contract, err := bindAccountActivation(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &AccountActivationFilterer{contract: contract}, nil
}

// bindAccountActivation binds a generic wrapper to an already deployed contract.
func bindAccountActivation(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(AccountActivationABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_AccountActivation *AccountActivationRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _AccountActivation.Contract.AccountActivationCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_AccountActivation *AccountActivationRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _AccountActivation.Contract.AccountActivationTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_AccountActivation *AccountActivationRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _AccountActivation.Contract.AccountActivationTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_AccountActivation *AccountActivationCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _AccountActivation.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_AccountActivation *AccountActivationTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _AccountActivation.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_AccountActivation *AccountActivationTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _AccountActivation.Contract.contract.Transact(opts, method, params...)
}

// Beneficiary is a free data retrieval call binding the contract method 0x38af3eed.
//
// Solidity: function beneficiary() view returns(address)
func (_AccountActivation *AccountActivationCaller) Beneficiary(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _AccountActivation.contract.Call(opts, &out, "beneficiary")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Beneficiary is a free data retrieval call binding the contract method 0x38af3eed.
//
// Solidity: function beneficiary() view returns(address)
func (_AccountActivation *AccountActivationSession) Beneficiary() (common.Address, error) {
	return _AccountActivation.Contract.Beneficiary(&_AccountActivation.CallOpts)
}

// Beneficiary is a free data retrieval call binding the contract method 0x38af3eed.
//
// Solidity: function beneficiary() view returns(address)
func (_AccountActivation *AccountActivationCallerSession) Beneficiary() (common.Address, error) {
	return _AccountActivation.Contract.Beneficiary(&_AccountActivation.CallOpts)
}

// NetworkCost is a free data retrieval call binding the contract method 0x1a5dcb1a.
//
// Solidity: function networkCost(string network) view returns(uint256)
func (_AccountActivation *AccountActivationCaller) NetworkCost(opts *bind.CallOpts, network string) (*big.Int, error) {
	var out []interface{}
	err := _AccountActivation.contract.Call(opts, &out, "networkCost", network)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NetworkCost is a free data retrieval call binding the contract method 0x1a5dcb1a.
//
// Solidity: function networkCost(string network) view returns(uint256)
func (_AccountActivation *AccountActivationSession) NetworkCost(network string) (*big.Int, error) {
	return _AccountActivation.Contract.NetworkCost(&_AccountActivation.CallOpts, network)
}

// NetworkCost is a free data retrieval call binding the contract method 0x1a5dcb1a.
//
// Solidity: function networkCost(string network) view returns(uint256)
func (_AccountActivation *AccountActivationCallerSession) NetworkCost(network string) (*big.Int, error) {
	return _AccountActivation.Contract.NetworkCost(&_AccountActivation.CallOpts, network)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_AccountActivation *AccountActivationCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _AccountActivation.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_AccountActivation *AccountActivationSession) Owner() (common.Address, error) {
	return _AccountActivation.Contract.Owner(&_AccountActivation.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_AccountActivation *AccountActivationCallerSession) Owner() (common.Address, error) {
	return _AccountActivation.Contract.Owner(&_AccountActivation.CallOpts)
}

// ActivateAccount is a paid mutator transaction binding the contract method 0x20b9f5bb.
//
// Solidity: function activateAccount(string network, string account) payable returns()
func (_AccountActivation *AccountActivationTransactor) ActivateAccount(opts *bind.TransactOpts, network string, account string) (*types.Transaction, error) {
	return _AccountActivation.contract.Transact(opts, "activateAccount", network, account)
}

// ActivateAccount is a paid mutator transaction binding the contract method 0x20b9f5bb.
//
// Solidity: function activateAccount(string network, string account) payable returns()
func (_AccountActivation *AccountActivationSession) ActivateAccount(network string, account string) (*types.Transaction, error) {
	return _AccountActivation.Contract.ActivateAccount(&_AccountActivation.TransactOpts, network, account)
}

// ActivateAccount is a paid mutator transaction binding the contract method 0x20b9f5bb.
//
// Solidity: function activateAccount(string network, string account) payable returns()
func (_AccountActivation *AccountActivationTransactorSession) ActivateAccount(network string, account string) (*types.Transaction, error) {
	return _AccountActivation.Contract.ActivateAccount(&_AccountActivation.TransactOpts, network, account)
}

// ChangeBeneficiary is a paid mutator transaction binding the contract method 0xdc070657.
//
// Solidity: function changeBeneficiary(address newBeneficiary) returns()
func (_AccountActivation *AccountActivationTransactor) ChangeBeneficiary(opts *bind.TransactOpts, newBeneficiary common.Address) (*types.Transaction, error) {
	return _AccountActivation.contract.Transact(opts, "changeBeneficiary", newBeneficiary)
}

// ChangeBeneficiary is a paid mutator transaction binding the contract method 0xdc070657.
//
// Solidity: function changeBeneficiary(address newBeneficiary) returns()
func (_AccountActivation *AccountActivationSession) ChangeBeneficiary(newBeneficiary common.Address) (*types.Transaction, error) {
	return _AccountActivation.Contract.ChangeBeneficiary(&_AccountActivation.TransactOpts, newBeneficiary)
}

// ChangeBeneficiary is a paid mutator transaction binding the contract method 0xdc070657.
//
// Solidity: function changeBeneficiary(address newBeneficiary) returns()
func (_AccountActivation *AccountActivationTransactorSession) ChangeBeneficiary(newBeneficiary common.Address) (*types.Transaction, error) {
	return _AccountActivation.Contract.ChangeBeneficiary(&_AccountActivation.TransactOpts, newBeneficiary)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_AccountActivation *AccountActivationTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _AccountActivation.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_AccountActivation *AccountActivationSession) RenounceOwnership() (*types.Transaction, error) {
	return _AccountActivation.Contract.RenounceOwnership(&_AccountActivation.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_AccountActivation *AccountActivationTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _AccountActivation.Contract.RenounceOwnership(&_AccountActivation.TransactOpts)
}

// SetNetworkCost is a paid mutator transaction binding the contract method 0x4f82e417.
//
// Solidity: function setNetworkCost(string network, uint256 cost) returns()
func (_AccountActivation *AccountActivationTransactor) SetNetworkCost(opts *bind.TransactOpts, network string, cost *big.Int) (*types.Transaction, error) {
	return _AccountActivation.contract.Transact(opts, "setNetworkCost", network, cost)
}

// SetNetworkCost is a paid mutator transaction binding the contract method 0x4f82e417.
//
// Solidity: function setNetworkCost(string network, uint256 cost) returns()
func (_AccountActivation *AccountActivationSession) SetNetworkCost(network string, cost *big.Int) (*types.Transaction, error) {
	return _AccountActivation.Contract.SetNetworkCost(&_AccountActivation.TransactOpts, network, cost)
}

// SetNetworkCost is a paid mutator transaction binding the contract method 0x4f82e417.
//
// Solidity: function setNetworkCost(string network, uint256 cost) returns()
func (_AccountActivation *AccountActivationTransactorSession) SetNetworkCost(network string, cost *big.Int) (*types.Transaction, error) {
	return _AccountActivation.Contract.SetNetworkCost(&_AccountActivation.TransactOpts, network, cost)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_AccountActivation *AccountActivationTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _AccountActivation.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_AccountActivation *AccountActivationSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _AccountActivation.Contract.TransferOwnership(&_AccountActivation.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_AccountActivation *AccountActivationTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _AccountActivation.Contract.TransferOwnership(&_AccountActivation.TransactOpts, newOwner)
}

// AccountActivationActivateAccountIterator is returned from FilterActivateAccount and is used to iterate over the raw logs and unpacked data for ActivateAccount events raised by the AccountActivation contract.
type AccountActivationActivateAccountIterator struct {
	Event *AccountActivationActivateAccount // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *AccountActivationActivateAccountIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AccountActivationActivateAccount)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(AccountActivationActivateAccount)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *AccountActivationActivateAccountIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AccountActivationActivateAccountIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AccountActivationActivateAccount represents a ActivateAccount event raised by the AccountActivation contract.
type AccountActivationActivateAccount struct {
	Network string
	Account string
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterActivateAccount is a free log retrieval operation binding the contract event 0x4113caa3b18a53c397c763a5e95ccc6d9d17072e52c5a13b3449d94cc4bdec54.
//
// Solidity: event ActivateAccount(string network, string account)
func (_AccountActivation *AccountActivationFilterer) FilterActivateAccount(opts *bind.FilterOpts) (*AccountActivationActivateAccountIterator, error) {

	logs, sub, err := _AccountActivation.contract.FilterLogs(opts, "ActivateAccount")
	if err != nil {
		return nil, err
	}
	return &AccountActivationActivateAccountIterator{contract: _AccountActivation.contract, event: "ActivateAccount", logs: logs, sub: sub}, nil
}

// WatchActivateAccount is a free log subscription operation binding the contract event 0x4113caa3b18a53c397c763a5e95ccc6d9d17072e52c5a13b3449d94cc4bdec54.
//
// Solidity: event ActivateAccount(string network, string account)
func (_AccountActivation *AccountActivationFilterer) WatchActivateAccount(opts *bind.WatchOpts, sink chan<- *AccountActivationActivateAccount) (event.Subscription, error) {

	logs, sub, err := _AccountActivation.contract.WatchLogs(opts, "ActivateAccount")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AccountActivationActivateAccount)
				if err := _AccountActivation.contract.UnpackLog(event, "ActivateAccount", log); err != nil {
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

// ParseActivateAccount is a log parse operation binding the contract event 0x4113caa3b18a53c397c763a5e95ccc6d9d17072e52c5a13b3449d94cc4bdec54.
//
// Solidity: event ActivateAccount(string network, string account)
func (_AccountActivation *AccountActivationFilterer) ParseActivateAccount(log types.Log) (*AccountActivationActivateAccount, error) {
	event := new(AccountActivationActivateAccount)
	if err := _AccountActivation.contract.UnpackLog(event, "ActivateAccount", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AccountActivationBeneficiaryChangedIterator is returned from FilterBeneficiaryChanged and is used to iterate over the raw logs and unpacked data for BeneficiaryChanged events raised by the AccountActivation contract.
type AccountActivationBeneficiaryChangedIterator struct {
	Event *AccountActivationBeneficiaryChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *AccountActivationBeneficiaryChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AccountActivationBeneficiaryChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(AccountActivationBeneficiaryChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *AccountActivationBeneficiaryChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AccountActivationBeneficiaryChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AccountActivationBeneficiaryChanged represents a BeneficiaryChanged event raised by the AccountActivation contract.
type AccountActivationBeneficiaryChanged struct {
	PreviousBeneficiary common.Address
	NewBeneficiary      common.Address
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterBeneficiaryChanged is a free log retrieval operation binding the contract event 0x768099735d1c322a05a5b9d7b76d99682a1833d3f7055e5ede25e0f2eeaa8c6d.
//
// Solidity: event BeneficiaryChanged(address indexed previousBeneficiary, address indexed newBeneficiary)
func (_AccountActivation *AccountActivationFilterer) FilterBeneficiaryChanged(opts *bind.FilterOpts, previousBeneficiary []common.Address, newBeneficiary []common.Address) (*AccountActivationBeneficiaryChangedIterator, error) {

	var previousBeneficiaryRule []interface{}
	for _, previousBeneficiaryItem := range previousBeneficiary {
		previousBeneficiaryRule = append(previousBeneficiaryRule, previousBeneficiaryItem)
	}
	var newBeneficiaryRule []interface{}
	for _, newBeneficiaryItem := range newBeneficiary {
		newBeneficiaryRule = append(newBeneficiaryRule, newBeneficiaryItem)
	}

	logs, sub, err := _AccountActivation.contract.FilterLogs(opts, "BeneficiaryChanged", previousBeneficiaryRule, newBeneficiaryRule)
	if err != nil {
		return nil, err
	}
	return &AccountActivationBeneficiaryChangedIterator{contract: _AccountActivation.contract, event: "BeneficiaryChanged", logs: logs, sub: sub}, nil
}

// WatchBeneficiaryChanged is a free log subscription operation binding the contract event 0x768099735d1c322a05a5b9d7b76d99682a1833d3f7055e5ede25e0f2eeaa8c6d.
//
// Solidity: event BeneficiaryChanged(address indexed previousBeneficiary, address indexed newBeneficiary)
func (_AccountActivation *AccountActivationFilterer) WatchBeneficiaryChanged(opts *bind.WatchOpts, sink chan<- *AccountActivationBeneficiaryChanged, previousBeneficiary []common.Address, newBeneficiary []common.Address) (event.Subscription, error) {

	var previousBeneficiaryRule []interface{}
	for _, previousBeneficiaryItem := range previousBeneficiary {
		previousBeneficiaryRule = append(previousBeneficiaryRule, previousBeneficiaryItem)
	}
	var newBeneficiaryRule []interface{}
	for _, newBeneficiaryItem := range newBeneficiary {
		newBeneficiaryRule = append(newBeneficiaryRule, newBeneficiaryItem)
	}

	logs, sub, err := _AccountActivation.contract.WatchLogs(opts, "BeneficiaryChanged", previousBeneficiaryRule, newBeneficiaryRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AccountActivationBeneficiaryChanged)
				if err := _AccountActivation.contract.UnpackLog(event, "BeneficiaryChanged", log); err != nil {
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

// ParseBeneficiaryChanged is a log parse operation binding the contract event 0x768099735d1c322a05a5b9d7b76d99682a1833d3f7055e5ede25e0f2eeaa8c6d.
//
// Solidity: event BeneficiaryChanged(address indexed previousBeneficiary, address indexed newBeneficiary)
func (_AccountActivation *AccountActivationFilterer) ParseBeneficiaryChanged(log types.Log) (*AccountActivationBeneficiaryChanged, error) {
	event := new(AccountActivationBeneficiaryChanged)
	if err := _AccountActivation.contract.UnpackLog(event, "BeneficiaryChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AccountActivationNetworkCostChangedIterator is returned from FilterNetworkCostChanged and is used to iterate over the raw logs and unpacked data for NetworkCostChanged events raised by the AccountActivation contract.
type AccountActivationNetworkCostChangedIterator struct {
	Event *AccountActivationNetworkCostChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *AccountActivationNetworkCostChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AccountActivationNetworkCostChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(AccountActivationNetworkCostChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *AccountActivationNetworkCostChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AccountActivationNetworkCostChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AccountActivationNetworkCostChanged represents a NetworkCostChanged event raised by the AccountActivation contract.
type AccountActivationNetworkCostChanged struct {
	Network string
	Cost    *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterNetworkCostChanged is a free log retrieval operation binding the contract event 0x4da674d59eb8b81bdf1fe3ef0a634f98c336dbd68c871570e5e9d5927ff020e2.
//
// Solidity: event NetworkCostChanged(string network, uint256 cost)
func (_AccountActivation *AccountActivationFilterer) FilterNetworkCostChanged(opts *bind.FilterOpts) (*AccountActivationNetworkCostChangedIterator, error) {

	logs, sub, err := _AccountActivation.contract.FilterLogs(opts, "NetworkCostChanged")
	if err != nil {
		return nil, err
	}
	return &AccountActivationNetworkCostChangedIterator{contract: _AccountActivation.contract, event: "NetworkCostChanged", logs: logs, sub: sub}, nil
}

// WatchNetworkCostChanged is a free log subscription operation binding the contract event 0x4da674d59eb8b81bdf1fe3ef0a634f98c336dbd68c871570e5e9d5927ff020e2.
//
// Solidity: event NetworkCostChanged(string network, uint256 cost)
func (_AccountActivation *AccountActivationFilterer) WatchNetworkCostChanged(opts *bind.WatchOpts, sink chan<- *AccountActivationNetworkCostChanged) (event.Subscription, error) {

	logs, sub, err := _AccountActivation.contract.WatchLogs(opts, "NetworkCostChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AccountActivationNetworkCostChanged)
				if err := _AccountActivation.contract.UnpackLog(event, "NetworkCostChanged", log); err != nil {
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

// ParseNetworkCostChanged is a log parse operation binding the contract event 0x4da674d59eb8b81bdf1fe3ef0a634f98c336dbd68c871570e5e9d5927ff020e2.
//
// Solidity: event NetworkCostChanged(string network, uint256 cost)
func (_AccountActivation *AccountActivationFilterer) ParseNetworkCostChanged(log types.Log) (*AccountActivationNetworkCostChanged, error) {
	event := new(AccountActivationNetworkCostChanged)
	if err := _AccountActivation.contract.UnpackLog(event, "NetworkCostChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AccountActivationOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the AccountActivation contract.
type AccountActivationOwnershipTransferredIterator struct {
	Event *AccountActivationOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *AccountActivationOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AccountActivationOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(AccountActivationOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *AccountActivationOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AccountActivationOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AccountActivationOwnershipTransferred represents a OwnershipTransferred event raised by the AccountActivation contract.
type AccountActivationOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_AccountActivation *AccountActivationFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*AccountActivationOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _AccountActivation.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &AccountActivationOwnershipTransferredIterator{contract: _AccountActivation.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_AccountActivation *AccountActivationFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *AccountActivationOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _AccountActivation.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AccountActivationOwnershipTransferred)
				if err := _AccountActivation.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_AccountActivation *AccountActivationFilterer) ParseOwnershipTransferred(log types.Log) (*AccountActivationOwnershipTransferred, error) {
	event := new(AccountActivationOwnershipTransferred)
	if err := _AccountActivation.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
