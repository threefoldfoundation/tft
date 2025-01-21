package multisig

import (
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/solana"
)

type StellarSignRequest struct {
	TxnXDR             string
	RequiredSignatures int
	Receiver           solana.Address // TODO: Valid ?
	Message            string         // Contains the deposit transaction hash in case of a refund
}

type StellarSignResponse struct {
	// Signature is a base64 of the signature
	Signature string
	// The account address
	Address string
}
