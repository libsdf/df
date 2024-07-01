package d1

import (
	"context"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/socks/tunnel"
	"net"
	// "github.com/libsdf/df/transport"
)

type ServerOptions struct {
	Port           int
	ProtocolParams conf.Values
	Handler        tunnel.ServerHandler
}

func Server(x context.Context, options *ServerOptions) error {
	addrStr := fmt.Sprintf(":%d", options.Port)
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	log.Debugf("<server> serving at %s...", addr)

	go func() {
		select {
		case <-x.Done():
			conn.Close()
		}
	}()

	chErr := make(chan error)
	idx := make(map[string]*packetConn)

	go func() {
		buf := make([]byte, 1024*16)
		for {
			size, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				chErr <- err
				return
			}
			if size <= 0 {
				continue
			}
			chunk := make([]byte, size)
			copy(chunk, buf[:size])
			connId := fmt.Sprintf("%s", remoteAddr)
			if c, found := idx[connId]; !found {
				// handshake
				print(c)
			}
		}
	}()

	select {
	case err := <-chErr:
		return err
	}
	return nil
}
