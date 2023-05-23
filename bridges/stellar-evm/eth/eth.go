package eth

import (
	"encoding/base64"
	"errors"

	"github.com/ethereum/go-ethereum/log"
)

const ERC20AddressLength = 20

type ERC20Address [ERC20AddressLength]byte

func GetErc20AddressFromB64(input string) (ethAddress ERC20Address, err error) {
	data, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		log.Warn("error decoding base64 input", "error", err.Error())
		return
	}

	// if the user sent an invalid memo, return the funds
	if len(data) != 20 {
		log.Warn("An ERC20 address should be 20 bytes")
		err = errors.New("An ERC20 address should be 20 bytes")
		return
	}

	copy(ethAddress[0:20], data)

	return ethAddress, nil
}
