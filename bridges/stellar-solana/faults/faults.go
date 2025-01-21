// Package faults defines errors for incorrect interaction with the bridge
package faults

import "errors"

var ErrInsufficientDepositAmount = errors.New("deposited amount is <= Fee")

var ErrInvalidReceiver = errors.New("receiver address does not exist, or is not a PDA which accepts our Mint")
