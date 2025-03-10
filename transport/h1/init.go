package h1

import (
	"context"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/socks/tunnel"
	"github.com/libsdf/df/transport"
)

func init() {
	transport.Register(&h1Suit{})
}

type h1Suit struct {
}

func (s *h1Suit) Name() string {
	return "h1"
}

func (s *h1Suit) Enabled() bool {
	return true
}

func (s *h1Suit) Server(cfg conf.Values) error {
	x := context.Background()

	go tunnel.CacheWorker(x)

	port := cfg.GetInt(conf.PORT)
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid serving port.")
	}
	options := &ServerOptions{
		Port:           port,
		ProtocolParams: cfg,
		Handler:        tunnel.NewServerHandler(),
	}
	return Server(x, options)
}

func (s *h1Suit) Client(cfg conf.Values, clientId string) (transport.Transport, error) {
	return CreateClient(cfg, clientId)
}
