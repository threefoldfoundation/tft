package solana

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// TestSubscribeTokenBurnsReconnects verifies that the burn subscription survives its websocket
// connection being closed underneath it: the client must be replaced by a fresh, working one.
//
// It needs a real Solana rpc/ws endpoint (host and path, without scheme) in SOLANA_RPC_ENDPOINT
// and is skipped otherwise.
func TestSubscribeTokenBurnsReconnects(t *testing.T) {
	endpoint := os.Getenv("SOLANA_RPC_ENDPOINT")
	if endpoint == "" {
		t.Skip("SOLANA_RPC_ENDPOINT not set")
	}

	keyFile := filepath.Join(t.TempDir(), "key.json")
	wallet := solana.NewWallet()
	if err := os.WriteFile(keyFile, []byte(keypairJSON(wallet.PrivateKey)), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sol, err := New(ctx, &SolanaConfig{
		KeyFile:      keyFile,
		NetworkName:  "production",
		TokenAddress: "TFTDVQ1mw85A79Zjeok5MQ1FNUQfgWd6FnPonApnrod",
		Endpoint:     endpoint,
	})
	if err != nil {
		t.Fatal(err)
	}

	burns, err := sol.SubscribeTokenBurns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Drain so a burn can never block the subscription goroutine; closed reports when the
	// subscription goroutine has exited and closed the channel.
	closed := make(chan struct{})
	go func() {
		for range burns {
		}
		close(closed)
	}()

	current := sol.ws()
	if current == nil {
		t.Fatal("no websocket client after subscribing")
	}

	// Kill the connection twice to make sure the recovery is not a one-shot
	for i := 1; i <= 2; i++ {
		// Simulates the provider dropping the connection: the client's read loop fails and hands the
		// error to the subscription, exactly as in production, and nothing else is cleaned up.
		current.Close()

		replaced := waitFor(t, 30*time.Second, func() bool {
			c := sol.ws()
			return c != nil && c != current
		})
		if !replaced {
			t.Fatalf("round %d: websocket client was not replaced after the connection was closed", i)
		}
		current = sol.ws()

		// The new client must actually work
		sub, err := current.LogsSubscribeMentions(sol.tokenAddress, rpc.CommitmentFinalized)
		if err != nil {
			t.Fatalf("round %d: new websocket client can not subscribe: %v", i, err)
		}
		sub.Unsubscribe()
		t.Logf("round %d: reconnected", i)
	}

	// Cancelling the context must stop the subscription goroutine promptly
	cancel()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("burn channel was not closed after the context was cancelled")
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return cond()
}

// keypairJSON renders a private key in the JSON byte-array format solana-keygen writes.
func keypairJSON(key solana.PrivateKey) string {
	out := "["
	for i, b := range key {
		if i > 0 {
			out += ","
		}
		out += strconv.Itoa(int(b))
	}
	return out + "]"
}
