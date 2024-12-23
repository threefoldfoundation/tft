package solana

// MintInfo about a mint to attempt for new tokens on the solana network
type MintInfo struct {
	// Amount of tokens in lamports
	Amount uint64
	// TxID transaction ID of the transaction which triggered this mint
	TxID string
	// To raw address bytes of the receiver of the tokens
	To [32]byte
}
