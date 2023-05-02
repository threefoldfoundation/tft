package bridge

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	pkgErrors "errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/support/errors"
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

func NewHost(ctx context.Context, secret, filteredID, relay string, psk string) (host.Host, routing.PeerRouting, error) {
	seed, err := strkey.Decode(strkey.VersionByteSeed, secret)
	if err != nil {
		return nil, nil, err
	}

	if len(seed) != ed25519.SeedSize {
		return nil, nil, fmt.Errorf("invalid seed size '%d' expecting '%d'", len(seed), ed25519.SeedSize)
	}

	sk := ed25519.NewKeyFromSeed(seed)

	privK, err := crypto.UnmarshalEd25519PrivateKey(sk)
	if err != nil {
		return nil, nil, err
	}

	var filteredPeerID peer.ID
	if filteredID != "" {
		filteredPeerID, err = getPeerIDFromStellarAddress(filteredID)
		if err != nil {
			return nil, nil, err
		}
	}

	return createLibp2pHost(ctx, privK, filteredPeerID, relay, psk)
}

func createLibp2pHost(ctx context.Context, privateKey crypto.PrivKey, filteredID peer.ID, relay string, psk string) (host.Host, routing.PeerRouting, error) {
	var idht *dht.IpfsDHT
	var err error

	cmgr, err := connmgr.NewConnManager(
		100, // Lowwater
		400, // HighWater,
	)
	if err != nil {
		return nil, nil, err
	}

	options := []config.Option{
		libp2p.Identity(privateKey),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",      // regular tcp connections
			"/ip4/0.0.0.0/udp/0/quic", // a UDP endpoint for the QUIC transport
		),
		libp2p.ConnectionManager(cmgr),
		// Attempt to open ports using uPNP for NATed hosts.
		libp2p.NATPortMap(),
		// Let this host use the DHT to find other hosts
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, err = dht.New(ctx, h, dht.Mode(dht.ModeAuto))
			return idht, err
		}),
	}

	//Explicitely set the transports to disable quic since it does not support private networks
	options = append(options, libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
	))

	if relay != "" {
		relayAddrInfo, err := peer.AddrInfoFromString(relay)
		if err != nil {
			return nil, nil, err
		}
		// Let this host use relays and advertise itself on relays if
		// it finds it is behind NAT. Use libp2p.Relay(options...) to
		// enable active relays and more.
		options = append(options, libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayAddrInfo}))
	}

	if psk != "" {
		key, err := hex.DecodeString(psk)
		if err != nil {
			return nil, nil, err
		}
		if len(psk) != 32 {
			return nil, nil, errors.New("psk must be 32 bytes long")
		}
		options = append(options, libp2p.PrivateNetwork(key))
	}

	if filteredID != "" {
		// filter on ID
		filter := NewGater(filteredID)
		options = append(options, libp2p.ConnectionGater(filter))
	}

	libp2phost, err := libp2p.New(options...)
	if err != nil {
		fmt.Println(err)
		return nil, nil, err
	}

	return libp2phost, idht, err
}

type SignersClient struct {
	peers  []peer.ID
	host   host.Host
	router routing.PeerRouting
	client *gorpc.Client
	relay  *peer.AddrInfo
}

type response struct {
	answer *SignResponse
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

func (s *SignersClient) Sign(ctx context.Context, signRequest SignRequest) ([]SignResponse, error) {

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

	var results []SignResponse

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

func (s *SignersClient) sign(ctx context.Context, id peer.ID, signRequest SignRequest) (*SignResponse, error) {
	if err := ConnectToPeer(ctx, s.host, s.router, s.relay, id); err != nil {
		return nil, errors.Wrapf(err, "failed to connect to host id '%s'", id.Pretty())
	}

	var response SignResponse
	if err := s.client.CallContext(ctx, id, "SignerService", "Sign", &signRequest, &response); err != nil {
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

func ConnectToPeer(ctx context.Context, p2phost host.Host, hostRouting routing.PeerRouting, relay *peer.AddrInfo, peerID peer.ID) (err error) {
	arHost := p2phost.(*autorelay.AutoRelayHost)

	findPeerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	peeraddrInfo, err := hostRouting.FindPeer(findPeerCtx, peerID)
	if err != nil {
		if pkgErrors.Is(err, routing.ErrNotFound) && relay != nil {
			targetMA, e := multiaddr.NewMultiaddr("/p2p/" + relay.ID.String() + "/p2p-circuit/p2p/" + peerID.String())
			if e != nil {
				err = e
				return
			}
			peeraddrInfo = peer.AddrInfo{
				ID:    peerID,
				Addrs: []multiaddr.Multiaddr{targetMA},
			}
		} else {
			return
		}
	}
	ConnectPeerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	err = arHost.Connect(ConnectPeerCtx, peeraddrInfo)
	return
}
