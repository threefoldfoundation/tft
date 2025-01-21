/*
Package p2p  has supporting libp2p functionality.
*/
package p2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"

	"github.com/stellar/go/strkey"
)

func GetPeerIDFromStellarAddress(address string) (peerID peer.ID, err error) {
	versionbyte, pubkeydata, err := strkey.DecodeAny(address)
	if err != nil {
		return
	}
	if versionbyte != strkey.VersionByteAccountID {
		err = fmt.Errorf("%s is not a valid Stellar address", address)
		return
	}
	libp2pPubKey, err := crypto.UnmarshalEd25519PublicKey(pubkeydata)
	if err != nil {
		return
	}

	peerID, err = peer.IDFromPublicKey(libp2pPubKey)
	return
}
func GetPeerIDsFromStellarAddresses(addresses []string) (ids []peer.ID, err error) {
	ids = make([]peer.ID, 0, len(addresses))
	for _, address := range addresses {
		id, err := GetPeerIDFromStellarAddress(address)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get peer info")
		}
		ids = append(ids, id)
	}
	return
}
