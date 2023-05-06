package main

import (
	"fmt"
	"os"

	"github.com/threefoldfoundation/tft/bridge/stellar/p2p"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage:", os.Args[0], "stellaraddress")
		os.Exit(1)
	}
	stellarAddress := os.Args[1]
	peerID, err := p2p.GetPeerIDFromStellarAddress(stellarAddress)
	if err != nil {
		panic(err)
	}
	fmt.Println("Peer ID:", peerID)

}
