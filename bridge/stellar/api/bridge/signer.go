package bridge

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/support/errors"
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
	//Force the relayfinder of the autorelay to start
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
	peers  []peer.ID
	host   host.Host
	router routing.PeerRouting
	client *gorpc.Client
	relay  *peer.AddrInfo
}

type response struct {
	answer *StellarSignResponse
	err    error
}

type ethResponse struct {
	answer *EthSignResponse
	err    error
}

// NewSignersClient creates a signer client with given stellar addresses
// the addresses are going to be used to get libp2p ID where we connect
// to and ask them to sign
func NewSignersClient(ctx context.Context, host host.Host, router routing.PeerRouting, addresses []string, relay *peer.AddrInfo) (*SignersClient, error) {
	var ids []peer.ID
	for _, address := range addresses {
		id, err := getPeerIDFromStellarAddress(address)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get peer info")
		}
		ids = append(ids, id)
	}

	return &SignersClient{
		client: gorpc.NewClient(host, Protocol),
		host:   host,
		router: router,
		peers:  ids,
		relay:  relay,
	}, nil
}

func (s *SignersClient) Sign(ctx context.Context, signRequest StellarSignRequest) ([]StellarSignResponse, error) {
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
			case ch <- response{answer: answer, err: err}:
			}
		}(addr, respCh)

	}

	var results []StellarSignResponse

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
					log.Error("failed to get signature from", "err", reply.err.Error())

				} else {
					if reply.answer != nil {
						log.Info("got a valid reply from a signer")
						results = append(results, *reply.answer)
					}
				}
				break responsechannelsLoop
			default: //don't block
			}
		}
		if receivedFrom >= 0 {
			//Remove the channel from the list
			responseChannels[receivedFrom] = responseChannels[len(responseChannels)-1]
			responseChannels = responseChannels[:len(responseChannels)-1]
			//check if we have enough signatures
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

func (s *SignersClient) sign(ctx context.Context, id peer.ID, signRequest StellarSignRequest) (*StellarSignResponse, error) {
	arHost := s.host.(*autorelay.AutoRelayHost)

	if err := client.ConnectToPeer(ctx, arHost, s.router, s.relay, id); err != nil {
		return nil, errors.Wrapf(err, "failed to connect to host id '%s'", id.Pretty())
	}

	var response StellarSignResponse
	if err := s.client.CallContext(ctx, id, "SignerService", "Sign", &signRequest, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *SignersClient) SignMint(ctx context.Context, signRequest EthSignRequest) ([]EthSignResponse, error) {
	// cancel context after 30 seconds
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	responseChannels := make([]chan ethResponse, 0, len(s.peers))
	for _, addr := range s.peers {
		respCh := make(chan ethResponse, 1)
		responseChannels = append(responseChannels, respCh)
		go func(peerID peer.ID, ch chan ethResponse) {
			defer close(ch)
			answer, err := s.signMint(ctxWithTimeout, peerID, signRequest)

			select {
			case <-ctxWithTimeout.Done():
			case ch <- ethResponse{answer: answer, err: err}:
			}
		}(addr, respCh)

	}

	var results []EthSignResponse

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
					log.Error("failed to get signature from", "err", reply.err.Error())

				} else {
					if reply.answer != nil {
						log.Info("got a valid reply from a signer")
						results = append(results, *reply.answer)
					}
				}
				break responsechannelsLoop
			default: //don't block
			}
		}
		if receivedFrom >= 0 {
			//Remove the channel from the list
			responseChannels[receivedFrom] = responseChannels[len(responseChannels)-1]
			responseChannels = responseChannels[:len(responseChannels)-1]
			//check if we have enough signatures
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

func (s *SignersClient) signMint(ctx context.Context, id peer.ID, signRequest EthSignRequest) (*EthSignResponse, error) {
	arHost := s.host.(*autorelay.AutoRelayHost)

	if err := client.ConnectToPeer(ctx, arHost, s.router, s.relay, id); err != nil {
		return nil, errors.Wrapf(err, "failed to connect to host id '%s'", id.Pretty())
	}

	var response EthSignResponse
	if err := s.client.CallContext(ctx, id, "SignerService", "SignMint", &signRequest, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func getPeerIDFromStellarAddress(address string) (peerID peer.ID, err error) {
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
	return peerID, err
}
