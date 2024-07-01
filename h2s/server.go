// Package h2s implements a http(s) proxy server.
// This proxy server uses a socks5 server as it's backend.
package h2s

import (
	"bufio"
	"context"
	"fmt"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/socks/socks5"
	"github.com/libsdf/df/utils"
	// "io"
	"net"
	"net/http"
	"strings"
)

func handleConn(cfg *Conf, conn net.Conn) {
	defer conn.Close()

	r := bufio.NewReader(conn)
	req, err := http.ReadRequest(r)
	if err != nil {
		if !utils.IsIOError(err) {
			log.Warnf("http.ReadRequest: %v", err)
		}
		return
	}

	// log.Debugf("proto=%s, method: %s", req.Proto, req.Method)
	// log.Debugf("uri: %s", req.URL.String())

	host := req.URL.Hostname()
	port := req.URL.Port()
	if len(port) <= 0 {
		switch strings.ToLower(req.URL.Scheme) {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}

	if len(port) <= 0 {
		if _, p, err := net.SplitHostPort(req.Host); err == nil {
			port = p
		}
	}

	if len(port) <= 0 {
		log.Warnf("invalid target port: %s", port)
		return
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	s5conn, err := socks5.Socks5Connect(cfg.ProxyAddr, addr)
	if err != nil {
		log.Warnf("Socks5Connect: %v", err)
		return
	}
	defer s5conn.Close()

	// if method is CONNECT
	if strings.EqualFold(req.Method, "connect") {
		resp := []byte("HTTP/1.1 200 OK\r\n\r\n")
		if _, err := conn.Write(resp); err != nil {
			log.Warnf("conn.Write: %v", err)
			return
		}
	} else {
		if err := req.Write(s5conn); err != nil {
			log.Warnf("req.Write: %v", err)
			return
		}
	}

	utils.Proxy(s5conn, conn)
}

// Server starts a http(s) proxy server using a socks5 proxy as backend.
func Server(x context.Context, cfg *Conf) error {
	if cfg == nil || cfg.Port <= 0 {
		log.Warnf("h2s server: invalid config.")
		return nil
	}
	addr := fmt.Sprintf(":%d", cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("net.Listen: %v", err)
	}
	defer ln.Close()
	log.Infof("serving http/https proxy at %s...", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Warnf("ln.Accept: %v", err)
		} else {
			go handleConn(cfg, conn)
		}
	}
}
