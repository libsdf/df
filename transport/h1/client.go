package h1

import (
	"bytes"
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

	workerAlive *atomic.Bool
	connected   *atomic.Bool

	// chR     chan []byte
	chW     chan []byte
	chClose chan int

	bufRead *utils.ReadWriteBuffer

	// bufRead    []byte
	// lenBufRead int
	// lckRead *sync.Mutex
}

func (c *Client) Connected() bool {
	return c.connected.Load() && c.workerAlive.Load()
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

func (c *Client) worker() {
	// log.Debugf("<cid:%s> worker start.", c.id)
	// defer log.Debugf("<cid:%s> worker exit.", c.id)
	defer c.connected.Store(false)

	c.workerAlive.Store(true)
	defer c.workerAlive.Store(false)

	// defer func() { c.chR <- nil }()

	psk := c.cfg.Get(conf.FRAMER_PSK)
	pskb, err := base64.StdEncoding.DecodeString(psk)
	if err != nil {
		log.Warnf("unable to base64 decode psk: %v", err)
		return
	}

	uri, err := url.Parse(c.serverUrl)
	if err != nil {
		log.Errorf("url.Parse(%s): %v", c.serverUrl, err)
		return
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
		log.Warnf("<cid:%s> net.Dial(%s): %v", c.id, addr, err)
		return
	}
	defer conn.Close()

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
		log.Errorf("<cid:%s> h.WriteTo: %v", c.id, err)
		return
	}

	// wait for response
	keyNewEnc := []byte{}
	if rs, err := http.ReadResponse(conn); err != nil {
		log.Errorf("<cid:%s> ReadResponse: %v", c.id, err)
		return
	} else {
		// log.Debugf("status: %d", rs.StatusCode)
		// for h, _ := range rs.Header {
		// 	log.Debugf("%s: %s", h, rs.Header.Get(h))
		// }
		keyNewStr := rs.Header.Get("X-Token")
		if len(keyNewStr) <= 0 {
			return
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
			return
		}
	} else {
		return
	}

	c.connected.Store(true)
	log.Debugf("<cid:%s> transport established.", c.id)

	framerCfg.Set(conf.FRAMER_TIMESTAMP, fmt.Sprintf("%d", ts))
	framerCfg.Set(conf.FRAMER_ROLE, "client")
	tx := (framer.Framer)(nil)
	if useWS {
		tx = ws.Framer(conn, framerCfg)
	} else {
		tx = f1.Framer(conn, framerCfg)
	}

	chErr := make(chan error)

	go func() {
		// maxBufSize := conf.MAX_BUFFER_SIZE
		buf := make([]byte, conf.DEFAULT_BUFFER_SIZE)
		// countFullRead := 0
		for {
			if size, err := tx.Read(buf); err != nil {
				if !utils.IsIOError(err) {
					log.Warnf("<cid:%s> tx.Read: %v", c.id, err)
				}
				chErr <- err
				return
			} else {
				// log.Debugf("<cid:%s> read %d bytes.", c.id, size)
				// chunk := make([]byte, size)
				// copy(chunk, buf[:size])
				// c.chR <- chunk

				if _, err := c.bufRead.Write(buf[:size]); err != nil {
					chErr <- err
					return
				}

				// // check if buf should be enlarged.
				// if size == bufSize {
				// 	countFullRead += 1
				// 	if countFullRead >= 10 && bufSize < maxBufSize {
				// 		bufSize *= 2
				// 		buf = make([]byte, bufSize)
				// 		// log.Debugf("buffer size set to %d", bufSize)
				// 	}
				// } else {
				// 	countFullRead = 0
				// }
			}
		}
	}()

	for {
		select {
		// case <- time.After(time.Second * 10):
		// 	//
		case chunk := <-c.chW:
			if len(chunk) > 0 {
				if _, err := tx.Write(chunk); err != nil {
					if !utils.IsIOError(err) {
						log.Warnf("tx.Write(%d bytes): %v",
							len(chunk), err,
						)
					}
					return
				} else {
					// log.Debugf("<cid:%s> wrote %d bytes.", c.id, size)
				}
			}
		case <-chErr:
			return
		case <-c.chClose:
			return
		}
	}
}

func (c *Client) Read(buf []byte) (int, error) {
	// c.lckRead.Lock()
	// defer c.lckRead.Unlock()

	if !c.workerAlive.Load() {
		// log.Warnf("<cid:%s> Read(): worker dead.", c.id)
		return 0, io.EOF
	}

	// log.Debugf("client.Read: enter.")
	// defer log.Debugf("client.Read: exit.")

	return c.bufRead.Read(buf)

	// if c.lenBufRead > 0 {
	// 	size := len(buf)
	// 	if size > c.lenBufRead {
	// 		size = c.lenBufRead
	// 	}
	// 	copy(buf[:size], c.bufRead[:size])
	// 	copy(c.bufRead, c.bufRead[size:])
	// 	c.lenBufRead -= size
	// 	return size, nil
	// }

	// select {
	// case <-time.After(time.Second * 5):
	// 	return 0, nil
	// case chunk := <-c.chR:
	// 	if chunk == nil {
	// 		return 0, io.EOF
	// 	}
	// 	size := len(chunk)
	// 	if len(chunk) > len(buf) {
	// 		size = len(buf)
	// 		copy(buf, chunk[:size])
	// 		if err := c.appendBuf(chunk[size:]); err != nil {
	// 			return size, err
	// 		}
	// 		return size, nil
	// 	}
	// 	copy(buf[:size], chunk)
	// 	return size, nil
	// }
}

func (c *Client) Write(buf []byte) (int, error) {
	if !c.workerAlive.Load() {
		// log.Warnf("worker dead. returns EOF.")
		return 0, io.EOF
	}
	c.chW <- buf
	return len(buf), nil
}

func (c *Client) Close() error {
	if closed := c.workerAlive.Swap(true); !closed {
		c.chClose <- 1
	}
	return nil
}

func CreateClient(cfg conf.Values, clientId string) (*Client, error) {
	if len(clientId) <= 0 {
		return nil, fmt.Errorf("empty client_id.")
	}

	serverUrl := cfg.Get(conf.SERVER_URL)
	if len(serverUrl) <= 0 {
		return nil, fmt.Errorf("invalid server_url.")
	}

	workerAlive := new(atomic.Bool)
	workerAlive.Store(true)

	c := &Client{
		cfg:       cfg,
		id:        clientId,
		serverUrl: serverUrl,

		workerAlive: workerAlive, // true,
		connected:   new(atomic.Bool),

		// chR:     make(chan []byte),
		chW:     make(chan []byte),
		chClose: make(chan int, 2),

		bufRead: utils.NewReadWriteBuffer(),
		// bufRead:    make([]byte, conf.DEFAULT_BUFFER_SIZE),
		// lenBufRead: 0,

		// lckRead: new(sync.Mutex),
	}
	go c.worker()

	return c, nil
}
