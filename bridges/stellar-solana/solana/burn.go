package solana

// Burn of tokens on solana. A burn is for an amount of tokens and carries a memo
type Burn struct {
	// Amount in the smallest unit
	amount uint64
	// Decimals for 1 full token
	decimals uint8

	// Memo attached to the Tx
	memo string
}

// Memo associated with the token burn
func (b Burn) Memo() string {
	return b.memo
}

// RawAmount of tokens burned in the smallest possible unit
func (b Burn) RawAmount() uint64 {
	return b.amount
}
