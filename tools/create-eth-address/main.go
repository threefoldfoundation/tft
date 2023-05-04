package main

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {

	key, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	fmt.Println("Address:", crypto.PubkeyToAddress(key.PublicKey))
	fmt.Println("Private key:", hex.EncodeToString(crypto.FromECDSA(key)))

}
