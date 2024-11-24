package h1pool

import (
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/transport"
	"github.com/libsdf/df/transport/h1"
	"github.com/libsdf/df/utils"
	"io"
	"sync"
	"sync/atomic"
	// "bufio"
	"context"
	"github.com/libsdf/df/log"
	"time"
)

var (
	lck = new(sync.RWMutex)
	// sharedClient = (*client)(nil)
	sharedClients = new(sync.Map)
	clients       = new(sync.Map)
)

/* a shared client transport */
type client struct {
	id             string
	tx             transport.Transport
	exited         *atomic.Bool
	lastActiveUnix *atomic.Int64
	createdAt      int64
	lck            *sync.Mutex
	bytesRecv      *atomic.Uint64
	bytesSent      *atomic.Uint64
}

func newClient(id string, tx transport.Transport) *client {
	now := time.Now().Unix()
	sc := &client{
		id:             id,
		tx:             tx,
		exited:         new(atomic.Bool),
		lastActiveUnix: new(atomic.Int64), // lastActiveUnix,
		createdAt:      now,
		lck:            new(sync.Mutex),
		bytesRecv:      new(atomic.Uint64),
		bytesSent:      new(atomic.Uint64),
	}
	sc.lastActiveUnix.Store(now)
	return sc
}

func (c *client) start() {
	log.Debugf("shared-client[%s] start", c.id)
	defer func() {
		log.Debugf(
			"shared-client[%s] exit. tx=%d, rx=%d",
			c.id,
			c.bytesSent.Load(),
			c.bytesRecv.Load(),
		)
	}()

	defer sharedClients.Delete(c.id)
	defer c.exited.Store(true)

	defer c.tx.Close()

	x, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		for {
			if p, err := readPacket(c.tx); err != nil {
				if err != io.EOF {
					log.Warnf("readPacket: %v", err)
				}
				return
			} else {
				c.lastActiveUnix.Store(time.Now().Unix())
				c.bytesRecv.Add(uint64(len(p.Data)))
				log.Debugf(
					"shared-client[%s] read packet[%s, %d bytes]",
					c.id, p.ClientId, len(p.Data),
				)
				if v, found := clients.Load(p.ClientId); found {
					cl := v.(*dividedClient)
					cl.writeBuf(p.Data)
				}
			}
		}

	}()

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
}

func (c *client) writePacket(p *Packet) error {
	log.Debugf("shared-client[%s] write packet[%s, %d bytes]",
		c.id, p.ClientId, len(p.Data),
	)
	c.lck.Lock()
	defer c.lck.Unlock()
	if err := writePacket(c.tx, p); err != nil {
		return err
	} else {
		c.bytesSent.Add(uint64(len(p.Data)))
	}
	return nil
}

func (c *client) WritePacket(p *Packet) error {
	c.lastActiveUnix.Store(time.Now().Unix())
	return c.writePacket(p)
}

// --------------------

type dividedClient struct {
	clientId string
	buf      *utils.ReadWriteBuffer
	serial   *atomic.Uint32
}

var _ transport.Transport = (*dividedClient)(nil)

func newDividedClient(clientId string) *dividedClient {
	return &dividedClient{
		clientId: clientId,
		buf:      utils.NewReadWriteBuffer(),
		serial:   new(atomic.Uint32),
	}
}

func (c *dividedClient) client() *client {
	// lastActiveUnix := int64(0)

	sc := (*client)(nil)
	latest := int64(0)

	sharedClients.Range(func(k, v interface{}) bool {
		c := v.(*client)
		if latest == 0 || latest < c.createdAt {
			latest = c.createdAt
			sc = c
		}
		// activeUnix := c.lastActiveUnix.Load()
		// if lastActiveUnix <= activeUnix {
		// 	lastActiveUnix = activeUnix
		// 	sc = c
		// }
		return true
	})

	return sc
}

func (c *dividedClient) Close() error {
	// log.Debugf("divClient[%s] closed and removed.", c.clientId)
	c.buf.Close()
	clients.Delete(c.clientId)
	return nil
}

// writeBuf used by sharedClient, feeding data it read.
func (c *dividedClient) writeBuf(dat []byte) {
	c.buf.Write(dat)
}

func (c *dividedClient) Read(buf []byte) (int, error) {
	if sc := c.client(); sc == nil {
		return 0, fmt.Errorf("sharedClient exited.")
	}

	return c.buf.Read(buf)
}

func (c *dividedClient) Write(dat []byte) (int, error) {
	sc := c.client()
	if sc == nil {
		return 0, fmt.Errorf("sharedClient exited.")
	}

	size := len(dat)
	serial := c.serial.Add(1) - 1
	p := &Packet{ClientId: c.clientId, Serial: serial, Data: dat}
	if err := sc.WritePacket(p); err != nil {
		return 0, err
	}
	return size, nil
}

//////////////////////

func getClient(cfg conf.Values, clientId string) (transport.Transport, error) {
	lck.Lock()
	defer lck.Unlock()

	now := time.Now().Unix()
	total := 0
	healthyCount := 0

	sharedClients.Range(func(k, v interface{}) bool {
		sc := v.(*client)
		total += 1
		if sc.createdAt > now-30 {
			healthyCount += 1
		}
		return true
	})

	if total < 5 && healthyCount == 0 {
		// no active shared clients. create a new
		trunkClientId := fmt.Sprintf("shared-%s", utils.UniqueId(8))
		if cl, err := h1.CreateClient(cfg, trunkClientId); err != nil {
			return nil, err
		} else {
			sc := newClient(trunkClientId, cl)
			sharedClients.Store(trunkClientId, sc)
			go sc.start()
			log.Debugf("shared-client[%s] added.", trunkClientId)
		}
	}

	// if sharedClient == nil || sharedClient.exited.Load() {
	// 	trunkClientId := fmt.Sprintf("shared-%s", utils.UniqueId(8))
	// 	if cl, err := h1.CreateClient(cfg, trunkClientId); err != nil {
	// 		return nil, err
	// 	} else {
	// 		sharedClient = newClient(cl)
	// 		go sharedClient.start()
	// 	}
	// }

	cl := newDividedClient(clientId)
	clients.Store(clientId, cl)

	// log.Debugf("divClient[%s] created.", clientId)

	return cl, nil
}
