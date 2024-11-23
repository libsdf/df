package h1pool

import (
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/transport"
	"github.com/libsdf/df/transport/h1"
	"github.com/libsdf/df/utils"
	"sync"
	"sync/atomic"
	// "bufio"
	"context"
	"github.com/libsdf/df/log"
	"time"
)

type client struct {
	tx transport.Transport
	//reader *bufio.Reader
	exited         *atomic.Bool
	lastActiveUnix *atomic.Int64
	lck            *sync.Mutex
}

func newClient(tx transport.Transport) *client {
	lastActiveUnix := new(atomic.Int64)
	lastActiveUnix.Store(time.Now().Unix())
	return &client{
		tx: tx,
		// reader: bufio.NewReader(tx),
		exited:         new(atomic.Bool),
		lastActiveUnix: lastActiveUnix,
		lck:            new(sync.Mutex),
	}
}

func (c *client) start() {
	log.Debugf("shared-client start")
	defer log.Debugf("shared-client exit.")
	defer c.exited.Store(true)

	x, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-time.After(time.Second * 15):
				// ping
				ping := &Packet{
					Op: OP_PING,
				}
				if err := c.writePacket(ping); err != nil {
					log.Debugf("writePacket(ping): %v", err)
					return
				}
				now := time.Now().Unix()
				if c.lastActiveUnix.Load()+60 < now {
					return
				}
			case <-x.Done():
				return
			}
		}
	}()

	for {
		if p, err := readPacket(c.tx); err != nil {
			log.Warnf("readPacket: %v", err)
			return
		} else {
			c.lastActiveUnix.Store(time.Now().Unix())
			log.Debugf(
				"shared-client read packet[%s, %d bytes]",
				p.ClientId, len(p.Data),
			)
			if v, found := clients.Load(p.ClientId); found {
				cl := v.(*dividedClient)
				cl.writeBuf(p.Data)
			}
		}
	}
}

func (c *client) writePacket(p *Packet) error {
	log.Debugf("shared-client write packet[%s, %d bytes]",
		p.ClientId, len(p.Data),
	)
	c.lck.Lock()
	defer c.lck.Unlock()
	if err := writePacket(c.tx, p); err != nil {
		return err
	}
	return nil
}

func (c *client) WritePacket(p *Packet) error {
	c.lastActiveUnix.Store(time.Now().Unix())
	return c.writePacket(p)
}

var (
	lck          = new(sync.RWMutex)
	sharedClient = (*client)(nil)
	clients      = new(sync.Map)
)

type dividedClient struct {
	clientId    string
	lck         *sync.RWMutex
	buf         []byte
	bufLen      int
	chBufUpdate chan int
	serial      *atomic.Uint32
}

var _ transport.Transport = (*dividedClient)(nil)

func newDividedClient(clientId string) *dividedClient {
	return &dividedClient{
		clientId:    clientId,
		lck:         new(sync.RWMutex),
		buf:         make([]byte, BUF_SIZE),
		bufLen:      0,
		chBufUpdate: make(chan int, 8),
		serial:      new(atomic.Uint32),
	}
}

func (c *dividedClient) Close() error {
	// log.Debugf("divClient[%s] closed and removed.", c.clientId)
	clients.Delete(c.clientId)
	return nil
}

// writeBuf used by sharedClient, feeding data it read.
func (c *dividedClient) writeBuf(dat []byte) {
	c.lck.Lock()
	defer c.lck.Unlock()

	size := len(dat)
	sizeAfter := c.bufLen + size
	if sizeAfter > cap(c.buf) {
		// extend the buffer
		capNew := cap(c.buf) + BUF_SIZE
		for {
			if capNew < sizeAfter {
				capNew += BUF_SIZE
			} else {
				break
			}
		}
		bufnew := make([]byte, capNew)
		copy(bufnew[:c.bufLen], c.buf[:c.bufLen])
		c.buf = bufnew
	}
	copy(c.buf[c.bufLen:sizeAfter], dat)
	c.bufLen = sizeAfter

	c.chBufUpdate <- sizeAfter
}

func (c *dividedClient) readFromBuf(buf []byte) int {
	c.lck.Lock()
	defer c.lck.Unlock()

	size := len(buf)
	if c.bufLen > 0 {
		if size > c.bufLen {
			size = c.bufLen
		}
		sizeRest := c.bufLen - size
		copy(buf[:size], c.buf[:size])
		copy(c.buf[0:sizeRest], c.buf[size:c.bufLen])
		c.bufLen = sizeRest
		return size
	}

	return 0
}

func (c *dividedClient) Read(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	if size := c.readFromBuf(buf); size > 0 {
		return size, nil
	}

	if sharedClient.exited.Load() {
		return 0, fmt.Errorf("sharedClient exited.")
	}

	for {
		select {
		case <-c.chBufUpdate:
			if size := c.readFromBuf(buf); size > 0 {
				return size, nil
			}
		}
	}
}

func (c *dividedClient) Write(dat []byte) (int, error) {
	if sharedClient.exited.Load() {
		return 0, fmt.Errorf("sharedClient exited.")
	}

	size := len(dat)
	serial := c.serial.Add(1) - 1
	p := &Packet{ClientId: c.clientId, Serial: serial, Data: dat}
	if err := sharedClient.WritePacket(p); err != nil {
		return 0, err
	}
	return size, nil
}

func getClient(cfg conf.Values, clientId string) (transport.Transport, error) {
	lck.Lock()
	defer lck.Unlock()

	if sharedClient == nil || sharedClient.exited.Load() {
		trunkClientId := fmt.Sprintf("shared-%s", utils.UniqueId(8))
		if cl, err := h1.CreateClient(cfg, trunkClientId); err != nil {
			return nil, err
		} else {
			sharedClient = newClient(cl)
			go sharedClient.start()
		}
	}

	cl := newDividedClient(clientId)
	clients.Store(clientId, cl)

	// log.Debugf("divClient[%s] created.", clientId)

	return cl, nil
}
