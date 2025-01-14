package bridge

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"time"

	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/rs/zerolog/log"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/support/errors"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/multisig"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/solana"
	"github.com/threefoldtech/libp2p-relay/client"
)

type SignerConfig struct {
	Secret   string
	BridgeID string
	Network  string
}

func (c *SignerConfig) Valid() error {
	if c.Network == "" {
		return fmt.Errorf("network is requires")
	}
	if c.Secret == "" {
		return fmt.Errorf("secret is required")
	}

	if c.BridgeID == "" {
		return fmt.Errorf("bridge identity is required")
	}

	return nil
}

func NewHost(ctx context.Context, secret, relay string, psk string) (host.Host, routing.PeerRouting, error) {
	seed, err := strkey.Decode(strkey.VersionByteSeed, secret)
	if err != nil {
		return nil, nil, err
	}

	if len(seed) != ed25519.SeedSize {
		return nil, nil, fmt.Errorf("invalid seed size '%d' expecting '%d'", len(seed), ed25519.SeedSize)
	}

	var privKey crypto.PrivKey
	if secret != "" {
		privKey, err = crypto.UnmarshalEd25519PrivateKey(
			ed25519.NewKeyFromSeed(seed),
		)
		if err != nil {
			return nil, nil, err
		}
	}

	key, err := hex.DecodeString(psk)
	if err != nil {
		return nil, nil, err
	}
	if len(key) != 32 {
		return nil, nil, errors.New("psk must be 32 bytes long")
	}

	relayAddrInfo, err := peer.AddrInfoFromString(relay)
	if err != nil {
		return nil, nil, err
	}

	ar, routing, err := client.CreateLibp2pHost(ctx, 0, true, key, privKey, []peer.AddrInfo{*relayAddrInfo})
	if err != nil {
		return nil, nil, err
	}
	// Force the relayfinder of the autorelay to start
	emitReachabilityChanged, err := ar.EventBus().Emitter(new(event.EvtLocalReachabilityChanged))
	if err != nil {
		return nil, nil, err
	}
	err = emitReachabilityChanged.Emit(event.EvtLocalReachabilityChanged{Reachability: network.ReachabilityUnknown})
	if err != nil {
		return nil, nil, err
	}

	return ar, routing, nil
}

type SignersClient struct {
	peers    []peer.ID
	host     host.Host
	router   routing.PeerRouting
	client   *gorpc.Client
	idClient *gorpc.Client
	relay    *peer.AddrInfo
}

type response struct {
	answer *multisig.StellarSignResponse
	peer   peer.ID
	err    error
}

type solanaResponse struct {
	answer *SolanaResponse
	peer   peer.ID
	err    error
}

type idResponse struct {
	answer *IDResponse
	peer   peer.ID
	err    error
}

// NewSignersClient creates a signer client to ask cosigners to sign
func NewSignersClient(host host.Host, router routing.PeerRouting, cosigners []peer.ID, relay *peer.AddrInfo) *SignersClient {
	return &SignersClient{
		client:   gorpc.NewClient(host, Protocol),
		idClient: gorpc.NewClient(host, SolIDProtocol),
		host:     host,
		router:   router,
		peers:    cosigners,
		relay:    relay,
	}
}

func (s *SignersClient) Sign(ctx context.Context, signRequest multisig.StellarSignRequest) ([]multisig.StellarSignResponse, error) {
	// cancel context after 30 seconds
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	responseChannels := make([]chan response, 0, len(s.peers))
	for _, addr := range s.peers {
		respCh := make(chan response, 1)
		responseChannels = append(responseChannels, respCh)
		go func(peerID peer.ID, ch chan response) {
			defer close(ch)
			answer, err := s.sign(ctxWithTimeout, peerID, signRequest)

			select {
			case <-ctxWithTimeout.Done():
			case ch <- response{answer: answer, peer: peerID, err: err}:
			}
		}(addr, respCh)

	}

	var results []multisig.StellarSignResponse

	for len(responseChannels) > 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		receivedFrom := -1
	responsechannelsLoop:
		for i, responseChannel := range responseChannels {
			select {
			case reply := <-responseChannel:
				receivedFrom = i
				if reply.err != nil {
					log.Error().Err(reply.err).Str("peerID", reply.peer.String()).Msg("failed to get signature")
				} else {
					if reply.answer != nil {
						log.Info().Str("peerID", reply.peer.String()).Msg("got a valid reply")
						results = append(results, *reply.answer)
					} else {
						log.Info().Str("peerID", reply.peer.String()).Msg("got empty signing request reply")
					}
				}
				break responsechannelsLoop
			default: // don't block
			}
		}
		if receivedFrom >= 0 {
			// Remove the channel from the list
			responseChannels[receivedFrom] = responseChannels[len(responseChannels)-1]
			responseChannels = responseChannels[:len(responseChannels)-1]
			// check if we have enough signatures
			if len(results) == signRequest.RequiredSignatures {
				break
			}
		} else {
			time.Sleep(time.Millisecond * 100)
		}

	}

	if len(results) != signRequest.RequiredSignatures {
		return nil, fmt.Errorf("required number of signatures is not met")
	}

	return results, nil
}

func (s *SignersClient) sign(ctx context.Context, id peer.ID, signRequest multisig.StellarSignRequest) (*multisig.StellarSignResponse, error) {
	arHost := s.host.(*autorelay.AutoRelayHost)

	if err := client.ConnectToPeer(ctx, arHost, s.router, s.relay, id); err != nil {
		return nil, errors.Wrapf(err, "failed to connect to host id '%s'", id)
	}

	var response multisig.StellarSignResponse
	log.Info().Str("PeerID", id.String()).Msg("Calling SignerService Sign")
	if err := s.client.CallContext(ctx, id, "SignerService", "Sign", &signRequest, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *SignersClient) SignMint(ctx context.Context, peers []peer.ID, signRequest SolanaRequest) ([]SolanaResponse, error) {
	// cancel context after 30 seconds
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	responseChannels := make([]chan solanaResponse, 0, len(s.peers))
	for _, addr := range peers {
		respCh := make(chan solanaResponse, 1)
		responseChannels = append(responseChannels, respCh)
		go func(peerID peer.ID, ch chan solanaResponse) {
			defer close(ch)
			answer, err := s.signMint(ctxWithTimeout, peerID, signRequest)

			select {
			case <-ctxWithTimeout.Done():
			case ch <- solanaResponse{answer: answer, peer: peerID, err: err}:
			}
		}(addr, respCh)

	}

	var results []SolanaResponse

	for len(responseChannels) > 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		receivedFrom := -1
	responsechannelsLoop:
		for i, responseChannel := range responseChannels {
			select {
			case reply := <-responseChannel:
				receivedFrom = i
				if reply.err != nil {
					log.Error().Err(reply.err).Str("peerID", reply.peer.String()).Msg("failed to get signature")
				} else {
					if reply.answer != nil {
						log.Info().Msg("got a valid reply from a signer")
						results = append(results, *reply.answer)
					}
				}
				break responsechannelsLoop
			default: // don't block
			}
		}
		if receivedFrom >= 0 {
			// Remove the channel from the list
			responseChannels[receivedFrom] = responseChannels[len(responseChannels)-1]
			responseChannels = responseChannels[:len(responseChannels)-1]
			// check if we have enough signatures
			if len(results) == int(signRequest.RequiredSignatures) {
				break
			}
		} else {
			time.Sleep(time.Millisecond * 100)
		}

	}

	if len(results) != int(signRequest.RequiredSignatures) {
		return nil, fmt.Errorf("required number of signatures is not met")
	}

	return results, nil
}

func (s *SignersClient) signMint(ctx context.Context, id peer.ID, signRequest SolanaRequest) (*SolanaResponse, error) {
	arHost := s.host.(*autorelay.AutoRelayHost)

	if err := client.ConnectToPeer(ctx, arHost, s.router, s.relay, id); err != nil {
		return nil, errors.Wrapf(err, "failed to connect to host id '%s'", id)
	}

	var response SolanaResponse
	if err := s.client.CallContext(ctx, id, "SignerService", "SignMint", &signRequest, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *SignersClient) SolID(ctx context.Context, requiredPeers int) (map[peer.ID]solana.Address, error) {
	// cancel context after 30 seconds
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	responseChannels := make([]chan idResponse, 0, len(s.peers))
	for _, addr := range s.peers {
		respCh := make(chan idResponse, 1)
		responseChannels = append(responseChannels, respCh)
		go func(peerID peer.ID, ch chan idResponse) {
			defer close(ch)
			answer, err := s.solID(ctxWithTimeout, peerID)

			select {
			case <-ctxWithTimeout.Done():
			case ch <- idResponse{answer: answer, peer: peerID, err: err}:
			}
		}(addr, respCh)

	}

	results := make(map[peer.ID]solana.Address)

	for len(responseChannels) > 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		receivedFrom := -1
	responsechannelsLoop:
		for i, responseChannel := range responseChannels {
			select {
			case reply := <-responseChannel:
				receivedFrom = i
				if reply.err != nil {
					log.Error().Err(reply.err).Str("peerID", reply.peer.String()).Msg("failed to get solana ID")
				} else {
					if reply.answer != nil {
						log.Info().Str("Peer ID", reply.peer.String()).Str("Solana ID", reply.answer.ID.String()).Msg("got a valid reply from a peer")
						results[reply.peer] = reply.answer.ID
					}
				}
				break responsechannelsLoop
			default: // don't block
			}
		}
		if receivedFrom >= 0 {
			// Remove the channel from the list
			responseChannels[receivedFrom] = responseChannels[len(responseChannels)-1]
			responseChannels = responseChannels[:len(responseChannels)-1]
			// check if we have enough signatures
			if len(results) == requiredPeers {
				break
			}
		} else {
			time.Sleep(time.Millisecond * 100)
		}

	}

	if len(results) < requiredPeers {
		return nil, fmt.Errorf("required number of peers not identified")
	}

	return results, nil
}

func (s *SignersClient) solID(ctx context.Context, id peer.ID) (*IDResponse, error) {
	arHost := s.host.(*autorelay.AutoRelayHost)

	if err := client.ConnectToPeer(ctx, arHost, s.router, s.relay, id); err != nil {
		return nil, errors.Wrapf(err, "failed to connect to host id '%s'", id)
	}

	var response IDResponse
	if err := s.idClient.CallContext(ctx, id, "SolIDService", "ID", &SolanaRequest{}, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
