package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage:", os.Args[0], "accountfile password")
		os.Exit(1)
	}
	accountFileName := os.Args[1]
	password := os.Args[2]
	keyjson, err := ioutil.ReadFile(accountFileName)
	if err != nil {
		panic(err)
	}
	key, err := keystore.DecryptKey(keyjson, password)
	if err != nil {
		panic(err)
	}
	rawPrivateKey := crypto.FromECDSA(key.PrivateKey)
	fmt.Println(hex.EncodeToString(rawPrivateKey))

}
