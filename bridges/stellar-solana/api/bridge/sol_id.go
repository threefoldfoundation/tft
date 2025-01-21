package bridge

import (
	"context"

	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog/log"
	"github.com/threefoldfoundation/tft/bridges/stellar-solana/solana"
)

const (
	SolIDProtocol = protocol.ID("/p2p/rpc/sol-id")
)

type SolIDService struct {
	id solana.Address
}

type IDRequest struct{}

type IDResponse struct {
	ID solana.Address
}

func NewSolIDServer(host host.Host, id solana.Address) error {
	server := gorpc.NewServer(host, SolIDProtocol)

	solIDService := SolIDService{
		id: id,
	}

	return server.Register(&solIDService)
}

func (sis *SolIDService) ID(ctx context.Context, req IDRequest, resp *IDResponse) error {
	log.Info().Msg("Solana ID request")

	resp.ID = sis.id

	return nil
}
