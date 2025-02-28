package solana

import (
	"sync"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rs/zerolog/log"
)

// transactionCache stores the results of a GetTransaction RPC call for a given signature.
type transactionCache struct {
	cache map[Signature]rpc.GetTransactionResult

	m sync.RWMutex
}

// newTransactionCache creates a new empty transaction cache.
func newTransactionCache() *transactionCache {
	return &transactionCache{
		cache: make(map[Signature]rpc.GetTransactionResult),
	}
}

// addTransaction to the cache.
func (tc *transactionCache) addTransaction(sig Signature, res rpc.GetTransactionResult) {
	tc.m.Lock()
	defer tc.m.Unlock()

	log.Debug().Str("sig", sig.String()).Msg("Adding solana transaction to cache")

	tc.cache[sig] = res
}

// getTransaction returns the transactionresult for the sig if it exists, otherwise returns nil.
func (tc *transactionCache) getTransaction(sig Signature) *rpc.GetTransactionResult {
	tc.m.RLock()
	defer tc.m.RUnlock()

	res, exists := tc.cache[sig]
	if !exists {
		return nil
	}

	return &res
}
