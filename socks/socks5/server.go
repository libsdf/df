package socks5

import (
	"bufio"
	"context"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/socks"
	"github.com/libsdf/df/socks/backend"
	"net"
)

type Options struct {
	Port            int
	ProtocolSuit    string
	ProtocolParams  conf.Values
	BackendProvider backend.BackendProvider
}

func handleConn(options *Options, x context.Context, conn net.Conn) {
	defer conn.Close()
	// log.Infof("%s connected.", conn.RemoteAddr())

	r := bufio.NewReader(conn)
	verb, err := r.Peek(1)
	if err != nil {
		return
	}
	ver := verb[0]

	proto := (socks.Protocol)(nil)

	switch ver {
	case 4:
	case 5:
		proto = newV5(options)
	default:
		// incompatible version
		return
	}

	if proto == nil {
		return
	}

	if err := proto.Handshake(r, conn); err != nil {
		return
	}

	if err := proto.HandleRequest(r, conn); err != nil {
		return
	}

}

func Server(x context.Context, options *Options) error {

	addr := fmt.Sprintf(":%d", options.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	defer ln.Close()
	log.Infof("serving at %s ...", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleConn(options, x, conn)
	}
}
