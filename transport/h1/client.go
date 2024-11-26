package h1

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/http"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/transport/framer"
	"github.com/libsdf/df/transport/framer/f1"
	"github.com/libsdf/df/transport/framer/ws"
	"github.com/libsdf/df/utils"
	"io"
	"net"
	"net/url"
	"strings"
	// "sync"
	"sync/atomic"
	"time"
)

type DialFunc func(string, string) (net.Conn, error)

type Client struct {
	cfg       conf.Values
	id        string
	serverUrl string

	closed    *atomic.Bool
	connected *atomic.Bool

	// chR     chan []byte
	chW     chan []byte
	chClose chan int

	bufRead *utils.ReadWriteBuffer

	// bufRead    []byte
	// lenBufRead int
	// lckRead *sync.Mutex
}

func (c *Client) Connected() bool {
	return c.connected.Load() && !c.closed.Load()
}

// func (c *Client) appendBuf(dat []byte) error {
// 	sizeAfter := c.lenBufRead + len(dat)
// 	if sizeAfter > cap(c.bufRead) {

// 		// extend the buffer
// 		sizeNew := cap(c.bufRead)
// 		for sizeNew < sizeAfter {
// 			sizeNew *= 2
// 		}
// 		if sizeNew > conf.MAX_BUFFER_SIZE*2 {
// 			return io.ErrShortBuffer
// 		}
// 		buf := make([]byte, sizeNew)

// 		copy(buf, c.bufRead[:c.lenBufRead])
// 		copy(buf[c.lenBufRead:], dat)
// 		c.bufRead = buf
// 		c.lenBufRead = sizeAfter
// 	} else {
// 		copy(c.bufRead[c.lenBufRead:], dat)
// 		c.lenBufRead = sizeAfter
// 	}
// 	return nil
// }

func (c *Client) Connect(x context.Context) (io.ReadWriteCloser, error) {
	// log.Debugf("<cid:%s> worker start.", c.id)
	// defer log.Debugf("<cid:%s> worker exit.", c.id)

	// if started := c.workerAlive.Swap(true); started {
	//     /* already started. */
	//     return nil, fmt.Errorf("already ")
	// }
	// defer c.workerAlive.Store(false)

	if c.closed.Load() {
		return nil, fmt.Errorf("Client already closed.")
	}
	// defer c.connected.Store(false)
	// defer func() { c.chR <- nil }()

	psk := c.cfg.Get(conf.FRAMER_PSK)
	pskb, err := base64.StdEncoding.DecodeString(psk)
	if err != nil {
		return nil, fmt.Errorf("unable to base64 decode psk: %v", err)
	}

	uri, err := url.Parse(c.serverUrl)
	if err != nil {
		return nil, fmt.Errorf("url.Parse(%s): %v", c.serverUrl, err)
	}

	scheme := strings.ToLower(uri.Scheme)

	useTLS := false
	host := uri.Hostname()
	port := uri.Port()

	useWS := false

	if scheme == "https" || scheme == "wss" {
		useTLS = true
	}
	if scheme == "ws" || scheme == "wss" {
		useWS = true
	}

	if len(port) <= 0 {
		switch scheme {
		case "http", "ws":
			port = "80"
		case "https", "wss":
			port = "443"
		}
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	dial := (DialFunc)(net.Dial)
	if useTLS {
		dial = func(network, addr string) (net.Conn, error) {
			tlsCfg := &tls.Config{}
			return tls.Dial(network, addr, tlsCfg)
		}
	}
	// log.Debugf("<client> connecting to %s...", addr)
	conn, err := dial("tcp4", addr)
	if err != nil {
		return nil, fmt.Errorf("<cid:%s> net.Dial(%s): %v", c.id, addr, err)
	}
	// defer conn.Close()

	// log.Debugf("<cid:%s> connected to transport-server %s.", c.id, addr)

	// construct authentication request.
	ts := time.Now().Unix()
	authBytes := make([]byte, 8+f1.KeySize())
	rand.Read(authBytes)
	binary.BigEndian.PutUint64(authBytes[:8], uint64(ts))
	authStr := ""
	if dat, err := f1.Encrypt(pskb, authBytes); err == nil {
		authStr = base64.StdEncoding.EncodeToString(dat)
	}

	h := http.NewHeader()
	q := make(url.Values)
	q.Set("c", c.id)
	uri.RawQuery = q.Encode()
	uri.Path = "/api/cv2"

	h.URL = uri
	h.Proto = "HTTP/1.1"
	if useWS {
		wsKeyb := make([]byte, 4)
		rand.Read(wsKeyb)
		wsKey := base64.StdEncoding.EncodeToString(wsKeyb)
		h.Method = "GET"
		h.Values.Set("Connection", "upgrade")
		h.Values.Set("Upgrade", "websocket")
		h.Values.Set("Sec-WebSocket-Key", wsKey)
	} else {
		h.Method = "POST"
		h.Values.Set("Content-Type", "image/png")
	}
	h.Values.Set("Authentication", authStr)
	headerBuf := bytes.NewBuffer([]byte{})
	h.WriteTo(headerBuf)

	// if _, err := h.WriteTo(tx); err != nil {
	if _, err := conn.Write(headerBuf.Bytes()); err != nil {
		return nil, fmt.Errorf("<cid:%s> h.WriteTo: %v", c.id, err)
	}

	// wait for response
	keyNewEnc := []byte{}
	if rs, err := http.ReadResponse(conn); err != nil {
		return nil, fmt.Errorf("<cid:%s> ReadResponse: %v", c.id, err)
	} else {
		// log.Debugf("status: %d", rs.StatusCode)
		// for h, _ := range rs.Header {
		// 	log.Debugf("%s: %s", h, rs.Header.Get(h))
		// }
		keyNewStr := rs.Header.Get("X-Token")
		if len(keyNewStr) <= 0 {
			return nil, fmt.Errorf("Empty key received.")
		}
		dat, err := base64.StdEncoding.DecodeString(keyNewStr)
		if err == nil {
			keyNewEnc = dat
		}
	}

	framerCfg := c.cfg.Clone()

	if len(keyNewEnc) > 0 {
		if keyNewRaw, err := f1.Decrypt(pskb, keyNewEnc); err == nil {
			keyNew := f1.Xor(keyNewRaw, authBytes[8:])
			pskNew := base64.StdEncoding.EncodeToString(keyNew)
			framerCfg.Set(conf.FRAMER_PSK, pskNew)
			// log.Debugf("<cid:%s> use new key: %x", c.id, keyNew)
		} else {
			return nil, fmt.Errorf("Decrypt: %v", err)
		}
	} else {
		return nil, fmt.Errorf("Empty key.")
	}

	c.connected.Store(true)
	log.Debugf("<cid:%s> connected.", c.id)

	framerCfg.Set(conf.FRAMER_TIMESTAMP, fmt.Sprintf("%d", ts))
	framerCfg.Set(conf.FRAMER_ROLE, "client")
	tx := (framer.Framer)(nil)
	if useWS {
		tx = ws.Framer(conn, framerCfg)
	} else {
		tx = f1.Framer(conn, framerCfg)
	}

	return tx, nil
}

func (c *Client) RelayTraffic(tx io.ReadWriteCloser) error {
	if c.closed.Load() {
		return fmt.Errorf("Client already closed.")
	}
	defer func() {
		c.closed.Store(true)
		log.Debugf("<cid:%s> stop relaying traffic.", c.id)
	}()

	chErr := make(chan error, 2)

	go func() {
		buf := make([]byte, conf.DEFAULT_BUFFER_SIZE)
		for {
			if size, err := tx.Read(buf); err != nil {
				if !utils.IsIOError(err) {
					chErr <- fmt.Errorf(
						"<cid:%s> tx.Read: %v", c.id, err,
					)
				} else {
					chErr <- err
				}
				return
			} else {
				if _, err := c.bufRead.Write(buf[:size]); err != nil {
					chErr <- fmt.Errorf("c.bufRead.Write: %v", err)
					return
				}
			}
		}
	}()

	for {
		select {
		case chunk := <-c.chW:
			if len(chunk) > 0 {
				if _, err := tx.Write(chunk); err != nil {
					// log.Warnf("tx.Write: %v", err)
					if !utils.IsIOError(err) {
						return fmt.Errorf("tx.Write(%d bytes): %v",
							len(chunk), err,
						)
					}
					return nil
				} else {
					// log.Debugf("<cid:%s> wrote %d bytes.", c.id, size)
				}
			}
		case err := <-chErr:
			// log.Warnf("%v", err)
			return err
		case <-c.chClose:
			return nil
		}
	}
}

func (c *Client) Read(buf []byte) (int, error) {
	if c.closed.Load() {
		log.Warnf("<cid:%s> Read(): client already closed.", c.id)
		return 0, io.EOF
	}
	return c.bufRead.Read(buf)
}

func (c *Client) Write(buf []byte) (int, error) {
	if c.closed.Load() {
		log.Warnf("<cid:%s> Write(): client already closed.", c.id)
		return 0, io.EOF
	}
	c.chW <- buf
	return len(buf), nil
}

func (c *Client) Close() error {
	log.Infof("<cid:%s> Close()", c.id)
	if closed := c.closed.Swap(true); !closed {
		c.chClose <- 1
	}
	return nil
}

// NewClient creates a new client. Please see CreateClient also.
func NewClient(cfg conf.Values, clientId string) (*Client, error) {
	if len(clientId) <= 0 {
		return nil, fmt.Errorf("empty client_id.")
	}

	serverUrl := cfg.Get(conf.SERVER_URL)
	if len(serverUrl) <= 0 {
		return nil, fmt.Errorf("invalid server_url.")
	}

	closed := new(atomic.Bool)
	closed.Store(false)

	c := &Client{
		cfg:       cfg,
		id:        clientId,
		serverUrl: serverUrl,

		closed:    closed,
		connected: new(atomic.Bool),

		chW:     make(chan []byte),
		chClose: make(chan int, 2),

		bufRead: utils.NewReadWriteBuffer(),
	}

	return c, nil
}

// CreateClient creates a client and start it.
// It's behaver is almost as same as:
//
//	cl, _ := NewClient(cfg, clientId)
//	if tx, err := cl.Connect(x); err == nil {
//	   cl.RelayTraffic(tx)
//	}
func CreateClient(cfg conf.Values, clientId string) (*Client, error) {
	c, err := NewClient(cfg, clientId)
	if err != nil {
		return nil, err
	}

	go func() {
		if tx, err := c.Connect(context.Background()); err != nil {
			if err != io.EOF {
				log.Warnf("%v", err)
			}
		} else {
			c.RelayTraffic(tx)
		}
	}()

	return c, nil
}
