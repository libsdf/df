package d1

import (
	"context"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/socks/tunnel"
	"github.com/libsdf/df/transport"
)

func init() {
	transport.Register(&d1Suit{})
}

type d1Suit struct {
}

func (s *d1Suit) Name() string {
	return "d1"
}

func (s *d1Suit) Enabled() bool {
	return true
}

func (s *d1Suit) Server(cfg conf.Values) error {
	port := cfg.GetInt(conf.PORT)
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid serving port.")
	}
	options := &ServerOptions{
		Port:           port,
		ProtocolParams: cfg,
		Handler:        tunnel.GetServerHandler(),
	}
	return Server(context.Background(), options)
}

func (s *d1Suit) Client(cfg conf.Values, clientId string) (transport.Transport, error) {
	return CreateClient(cfg, clientId)
}
