package stellar

import "errors"

var ErrAccountAlreadyExists = errors.New("Account already exists")
var ErrInvalidAddress = errors.New("Invalid Stellar address")
