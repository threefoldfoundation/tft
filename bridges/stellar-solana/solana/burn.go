package solana

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var (
	ErrBadIxLen    = errors.New("transaction has more than 3 or less than 2 instructions")
	ErrInvalidBurn = errors.New("malformed burn transaction")
	ErrTxUnsigned  = errors.New("transaction has no signatures")
)

// Burn of tokens on solana. A burn is for an amount of tokens and carries a memo
type Burn struct {
	// Amount in lamports
	amount uint64
	// Decimals for 1 full token
	decimals uint8

	// Memo attached to the Tx
	memo string

	// Account which called the burn operation on Solana
	// TODO: fill in
	caller Address

	// Heigt of the block? the tx was part of
	// TODO: fill in
	// blockHeight uint64

	// signature of the transaction, which is also the txId
	signature Signature
}

// Memo associated with the token burn
func (b Burn) Memo() string {
	return b.memo
}

// RawAmount of tokens burned in lamports
func (b Burn) RawAmount() uint64 {
	return b.amount
}

// TxID of the burn transaction
func (b Burn) TxID() Signature {
	return b.signature
}

// ShortTxID of the transaction
func (b Burn) ShortTxID() ShortTxID {
	return shortenTxID(b.signature)
}

// Caller of the burn operation
func (b Burn) Caller() Address {
	return b.caller
}

// BlockHeight the tx was included at
// func (b Burn) BlockHeight() uint64 {
// 	return b.blockHeight
// }

func burnFromTransaction(tx solana.Transaction) (Burn, error) {
	// Compute limit is optional
	ixLen := len(tx.Message.Instructions)
	if ixLen > 3 || ixLen < 2 {
		return Burn{}, ErrBadIxLen
	}

	memoText := ""
	burnAmount := uint64(0)
	tokenDecimals := uint8(0)
	illegalOp := false

outer:
	for _, ix := range tx.Message.Instructions {
		accounts, err := tx.AccountMetaList()
		if err != nil {
			return Burn{}, errors.Wrap(err, "could not resolve account meta list")
		}

		switch tx.Message.AccountKeys[ix.ProgramIDIndex] {
		case memoProgram:
			// TODO: verify encoding
			if len(ix.Data) == 0 {
				log.Debug().Msg("Empty memo instruction")
				illegalOp = true
				break
			}
			memoText = string(ix.Data[:])
		case tokenProgram2022:
			tokenIx, err := token.DecodeInstruction(accounts, ix.Data)
			if err != nil {
				illegalOp = true
				break
			}

			// At this point, verify its a burn

			switch burn := tokenIx.Impl.(type) {
			case *token.BurnChecked:
				if burn.Amount == nil {
					illegalOp = true
					break outer
				}
				if burn.Decimals == nil {
					illegalOp = true
					break outer
				}
				burnAmount = *burn.Amount
				tokenDecimals = *burn.Decimals

			// TODO: Should we allow this or only allow BurnChecked
			// case *token.Burn:
			// 	if burn.Amount == nil {
			// 		illegalOp = true
			// 		break outer
			// 	}
			// 	if burn.Decimals == nil {
			// 		illegalOp = true
			// 		break outer
			// 	}
			// 	burnAmount = *burn.Amount
			// 	tokenDecimals = *burn.Decimals

			default:
				illegalOp = true
				break outer
			}
		case computeBudgetProgram:
		// Nothing really to do here, we only care that this is ineed a compute budget program ix
		default:
			// We don't allow for other instructions at this time, so this condition is terminal for the tx validation.
			illegalOp = true
			break outer

		}
	}

	if len(tx.Signatures) == 0 {
		return Burn{}, ErrTxUnsigned
	}

	if memoText != "" && burnAmount != 0 && !illegalOp {
		return Burn{amount: burnAmount, decimals: tokenDecimals, memo: memoText, signature: tx.Signatures[0]}, nil
	}

	return Burn{}, ErrInvalidBurn
}
