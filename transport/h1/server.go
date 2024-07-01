package h1

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/http"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/transport/framer/f1"
	"github.com/libsdf/df/transport/framer/ws"
	"github.com/libsdf/df/utils"
	"io"
	"net"
	"strings"
	"time"
)

const (
	wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

type ServerOptions struct {
	Port           int
	ProtocolParams conf.Values
	Handler        func(string, io.ReadWriteCloser)
}

func Server(x context.Context, options *ServerOptions) error {
	addr := fmt.Sprintf(":%d", options.Port)
	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		return err
	}

	log.Debugf("<server> serving at %s...", addr)

	go func() {
		select {
		case <-x.Done():
			ln.Close()
		}
	}()

	for {
		if conn, err := ln.Accept(); err != nil {
			return err
		} else {
			go handleServerConn(options, conn)
		}
	}
}

func abort(h *http.Header, conn net.Conn) {
	rs := bytes.NewBuffer([]byte{})
	body := []byte("denied")
	fmt.Fprintf(rs, "%s 403 DENIED\r\n", h.Proto)
	fmt.Fprintf(rs, "Connection: close\r\n")
	fmt.Fprintf(rs, "Content-Type: text/plain\r\n")
	fmt.Fprintf(rs, "Content-Length: %d\r\n", len(body))
	fmt.Fprintf(rs, "\r\n")
	rs.Write(body)
	if _, err := conn.Write(rs.Bytes()); err != nil {
		if !utils.IsIOError(err) {
			log.Warnf("conn.Write(resposne-403): %v", err)
		}
	}
}

func handleServerConn(options *ServerOptions, conn net.Conn) {
	defer conn.Close()

	hr := http.NewHeaderReader(conn)
	header, err := hr.PeekHeader()
	if err != nil {
		return
	}

	clientId := header.URL.Query().Get("c")
	// log.Infof("clientId: %s", clientId)
	if len(clientId) <= 0 {
		abort(header, conn)
		return
	}

	psk := options.ProtocolParams.Get(conf.FRAMER_PSK)
	pskb, err := base64.StdEncoding.DecodeString(psk)
	if err != nil {
		log.Warnf("unable to base64 decode psk: %v", err)
		abort(header, conn)
		return
	}

	authRequired := true
	authBytes := ([]byte)(nil)
	ts := int64(0)

	if authRequired {
		authStr := header.Values.Get("Authentication")
		if len(authStr) > 128 || len(authStr) <= 0 {
			abort(header, conn)
			return
		}
		authBytesEnc, err := base64.StdEncoding.DecodeString(authStr)
		if err != nil {
			abort(header, conn)
			return
		}

		authb, err := f1.Decrypt(pskb, authBytesEnc)
		if err != nil {
			log.Warnf("f1.Decrypt: %v", err)
			abort(header, conn)
			return
		}

		authBytesLen := 8 + f1.KeySize()
		if len(authb) < authBytesLen {
			abort(header, conn)
			return
		}

		ts = int64(binary.BigEndian.Uint64(authb[:8]))
		now := time.Now().Unix()
		if ts-now > 120 || ts-now < -120 {
			abort(header, conn)
			return
		}

		authBytes = authb
	} else {
		return
	}

	keyNewRaw := f1.NewKey()
	keyNewEnc, err := f1.Encrypt(pskb, keyNewRaw)
	if err != nil {
		log.Warnf("f1.Encrypt: %v", err)
		abort(header, conn)
		return
	}
	keyNewEncStr := base64.StdEncoding.EncodeToString(keyNewEnc)

	// check if the request demands a websocket handshake.
	isWs := false
	wsAccept := ""
	if strings.EqualFold(header.Values.Get("Upgrade"), "websocket") {
		isWs = true
		wsKey := header.Values.Get("Sec-WebSocket-Key")
		if len(wsKey) > 0 {
			wsAckStr := fmt.Sprintf("%s%s", wsKey, wsGUID)
			wsAckb := sha1.Sum([]byte(wsAckStr))
			wsAccept = base64.StdEncoding.EncodeToString(wsAckb[:])
		}
	}

	// send a response
	rs := bytes.NewBuffer([]byte{})
	if isWs {
		wsProto := header.Values.Get("Sec-WebSocket-Protocol")
		fmt.Fprintf(rs, "%s 101 Switching Protocols\r\n", header.Proto)
		fmt.Fprintf(rs, "Connection: Upgrade\r\n")
		fmt.Fprintf(rs, "Upgrade: websocket\r\n")
		fmt.Fprintf(rs, "Sec-WebSocket-Accept: %s\r\n", wsAccept)
		if len(wsProto) > 0 {
			fmt.Fprintf(rs, "Sec-WebSocket-Protocol: %s\r\n", wsProto)
		}
		fmt.Fprintf(rs, "Sec-WebSocket-Version: 7\r\n")
	} else {
		fmt.Fprintf(rs, "%s 200 OK\r\n", header.Proto)
		fmt.Fprintf(rs, "Connection: close\r\n")
		fmt.Fprintf(rs, "Content-Type: image/png,stream=1\r\n")
		fmt.Fprintf(rs, "Content-Transfer-Encoding: custom\r\n")
	}
	fmt.Fprintf(rs, "X-Token: %s\r\n", keyNewEncStr)
	fmt.Fprintf(rs, "\r\n")

	if _, err := conn.Write(rs.Bytes()); err != nil {
		if !utils.IsIOError(err) {
			log.Warnf("conn.Write(resposne): %v", err)
		}
		return
	}

	keyNew := f1.Xor(keyNewRaw, authBytes[8:])
	keyNewStr := base64.StdEncoding.EncodeToString(keyNew)
	params := options.ProtocolParams.Clone()
	params.Set(conf.FRAMER_PSK, keyNewStr)

	// log.Debugf("<cid:%s> offering new psk: %s", clientId, keyNewStr)

	if options.Handler != nil {
		params.Set(conf.FRAMER_TIMESTAMP, fmt.Sprintf("%d", ts))
		params.Set(conf.FRAMER_ROLE, "server")

		if isWs {
			tx := ws.Framer(conn, params)
			options.Handler(clientId, tx)
		} else {
			tx := f1.Framer(conn, params)
			options.Handler(clientId, tx)
		}

		return
	}
}
