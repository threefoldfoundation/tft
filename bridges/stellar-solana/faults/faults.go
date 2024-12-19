// Package faults defines errors for incorrect interaction with the bridge
package faults

import "errors"

var ErrInsufficientDepositAmount = errors.New("deposited amount is <= Fee")
