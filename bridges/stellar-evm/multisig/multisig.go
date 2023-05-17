package multisig

import "github.com/ethereum/go-ethereum/common"

type StellarSignRequest struct {
	TxnXDR             string
	RequiredSignatures int
	Receiver           common.Address //TODO: How can this be an Ethereum common.Address ?
	Block              uint64
	Message            string
}

type StellarSignResponse struct {
	// Signature is a base64 of the signature
	Signature string
	// The account address
	Address string
}
