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
	lck           = new(sync.RWMutex)
	workerAlive   = new(atomic.Bool)
	sharedClients = new(sync.Map)
	clients       = new(sync.Map)
)

func queryClientInfo(id string) (isOldest bool, total int) {
	total = 0
	oldest := int64(0)
	oldestId := ""
	sharedClients.Range(func(k, v interface{}) bool {
		sc := v.(*client)
		total += 1
		if oldest == 0 || oldest > sc.createdAt {
			oldest = sc.createdAt
			oldestId = sc.id
		}
		return true
	})
	if oldestId == id {
		isOldest = true
	}
	return isOldest, total
}

/* a shared client transport */
type client struct {
	id           string
	tx           *h1.Client // transport.Transport
	exited       *atomic.Bool
	lastPongUnix *atomic.Int64
	lastPingUnix *atomic.Int64
	lastDataUnix *atomic.Int64
	createdAt    int64
	lck          *sync.Mutex
	bytesRecv    *atomic.Uint64
	bytesSent    *atomic.Uint64
}

func newClient(id string, tx *h1.Client) *client {
	now := time.Now().Unix()
	sc := &client{
		id:           id,
		tx:           tx,
		exited:       new(atomic.Bool),
		lastPongUnix: new(atomic.Int64),
		lastPingUnix: new(atomic.Int64),
		lastDataUnix: new(atomic.Int64),
		createdAt:    now,
		lck:          new(sync.Mutex),
		bytesRecv:    new(atomic.Uint64),
		bytesSent:    new(atomic.Uint64),
	}
	return sc
}

func (c *client) start() {
	log.Infof("shared-client[%s] start", c.id)
	defer func() {
		log.Infof(
			"shared-client[%s] exit. tx=%d, rx=%d",
			c.id,
			c.bytesSent.Load(),
			c.bytesRecv.Load(),
		)
		now := time.Now().Unix()
		log.Infof("shared-clients:")
		sharedClients.Range(func(k, v interface{}) bool {
			id := k.(string)
			sc := v.(*client)
			log.Infof(" ~ [%s] %d", id, sc.createdAt-now)
			return true
		})
	}()

	defer sharedClients.Delete(c.id)
	defer c.exited.Store(true)

	x, cancel := context.WithCancel(context.Background())

	if tx, err := c.tx.Connect(x); err != nil {
		log.Warnf("c.tx.Connect: %v", err)
		return
	} else {
		defer c.tx.Close()
		go c.tx.RelayTraffic(tx)
	}

	log.Infof("shared-client[%s] connected.", c.id)

	// chErr := make(chan error)
	// go func(){
	// 	chErr <- c.tx.Start(x)
	// }()

	go func() {
		defer cancel()
		defer log.Debugf("shared-client[%s] stopped reading.", c.id)
		for {
			if p, err := readPacket(c.tx); err != nil {
				if err != io.EOF {
					log.Warnf("readPacket: %v", err)
				} else {
					log.Debugf("readPacket: %v", err)
				}
				return
			} else {
				// c.lastRecvUnix.Store(time.Now().Unix())
				c.bytesRecv.Add(uint64(len(p.Data)))
				log.Debugf(
					"shared-client[%s] read packet[%s, %d bytes]",
					c.id, p.ClientId, len(p.Data),
				)
				switch p.Op {
				case OP_DATA:
					c.lastDataUnix.Store(time.Now().Unix())
					if v, found := clients.Load(p.ClientId); found {
						cl := v.(*dividedClient)
						cl.writeBuf(p.Data)
					}
				case OP_PING:
					// unlikely.
				case OP_PONG:
					c.lastPongUnix.Store(time.Now().Unix())
				case OP_EOS:
					log.Infof("EOS received for %s.", p.ClientId)
					if v, found := clients.Load(p.ClientId); found {
						cl := v.(*dividedClient)
						cl.Close()
					}
				}
			}
		}

	}()

	serial := uint64(1)
	for {
		select {
		case <-time.After(time.Second * 5):
			now := time.Now().Unix()
			lastPing := c.lastPingUnix.Load()
			lastPong := c.lastPongUnix.Load()
			if lastPing > c.createdAt && lastPong+15 < lastPing {
				log.Warnf(
					"shared-client[%s] pong stalled. %d-%d=%d",
					c.id, lastPing, lastPong, lastPing-lastPong,
				)
				return
			}
			if c.createdAt+120 < now {
				idle := c.lastDataUnix.Load()+30 < now
				idleTooLong := c.lastDataUnix.Load()+60 < now
				oldest, total := queryClientInfo(c.id)
				if oldest && ((total > 1 && idle) || idleTooLong) {
					log.Infof("shared-client[%s] decommission.", c.id)
					return
				}
			}
			if serial%2 == 0 {
				// ping
				ping := &Packet{
					Op: OP_PING,
				}
				if err := c.writePacket(ping); err != nil {
					log.Debugf("writePacket(ping): %v", err)
					return
				} else {
					c.lastPingUnix.Store(time.Now().Unix())
				}
			}
			serial += 1
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
		if p.Op == OP_DATA {
			c.bytesSent.Add(uint64(len(p.Data)))
			c.lastDataUnix.Store(time.Now().Unix())
		}
	}
	return nil
}

func (c *client) WritePacket(p *Packet) error {
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

func (c *dividedClient) getAvailableSharedClient() *client {
	scAny := (*client)(nil)
	sc := (*client)(nil)
	latest := int64(0)

	sharedClients.Range(func(k, v interface{}) bool {
		c := v.(*client)
		scAny = c
		if latest == 0 || latest < c.createdAt {
			if c.tx.Connected() {
				latest = c.createdAt
				sc = c
			} else if sc == nil {
				sc = c
			}
		}
		return true
	})

	if sc != nil {
		return sc
	}

	return scAny
}

func (c *dividedClient) client() *client {
	if sc := c.getAvailableSharedClient(); sc != nil {
		return sc
	}
	dur := time.Duration(0)
	timeout := false
	for {
		select {
		case <-time.After(time.Second * 2):
			if sc := c.getAvailableSharedClient(); sc != nil {
				return sc
			}
			dur += time.Second * 2
			if dur > time.Second*30 {
				timeout = true
				break
			}
		}
		if timeout {
			break
		}
	}
	return nil
}

func (c *dividedClient) Close() error {
	// log.Debugf("divClient[%s] closed and removed.", c.clientId)
	c.buf.Close()
	clients.Delete(c.clientId)

	sc := c.client()
	if sc != nil {
		serial := c.serial.Add(1) - 1
		p := &Packet{ClientId: c.clientId, Serial: serial, Op: OP_EOS}
		if err := sc.WritePacket(p); err != nil {
			// ignore?
		}
	}

	return nil
}

// writeBuf used by sharedClient, feeding data it read.
func (c *dividedClient) writeBuf(dat []byte) {
	c.buf.Write(dat)
}

func (c *dividedClient) Read(buf []byte) (int, error) {
	if sc := c.client(); sc == nil {
		return 0, fmt.Errorf("no alive transport.")
	}

	return c.buf.Read(buf)
}

func (c *dividedClient) Write(dat []byte) (int, error) {
	sc := c.client()
	if sc == nil {
		return 0, fmt.Errorf("no alive transport.")
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

var sharedClientIdPrefix = fmt.Sprintf("s-%s", utils.UniqueId(8))

func tryAddNewSharedClient(cfg conf.Values) error {
	lck.Lock()
	defer lck.Unlock()

	now := time.Now().Unix()
	total := 0
	healthyCount := 0

	sharedClients.Range(func(k, v interface{}) bool {
		sc := v.(*client)
		total += 1
		// idle := sc.lastDataUnix.Load() + 30 < now
		if sc.createdAt > now-300 {
			healthyCount += 1
		}
		return true
	})

	if total < 5 && healthyCount == 0 {
		// no active shared clients. create a new
		// trunkClientId := fmt.Sprintf("shared-%s", utils.UniqueId(8))
		trunkClientId := fmt.Sprintf(
			"%s:%s",
			sharedClientIdPrefix,
			utils.UniqueId(8),
		)
		if cl, err := h1.NewClient(cfg, trunkClientId); err != nil {
			return err
		} else {
			sc := newClient(trunkClientId, cl)
			sharedClients.Store(trunkClientId, sc)
			go sc.start()
			log.Debugf("shared-client[%s] added.", trunkClientId)
		}
	}

	return nil
}

func clientsWorker(cfg conf.Values) {
	if alive := workerAlive.Swap(true); alive {
		return
	}
	defer workerAlive.Store(false)
	for {
		select {
		case <-time.After(time.Second * 2):
			hasClients := false
			clients.Range(func(_k, _v interface{}) bool {
				hasClients = true
				return false
			})
			if hasClients {
				hasTxs := false
				sharedClients.Range(func(_k, _v interface{}) bool {
					hasTxs = true
					return false
				})
				if !hasTxs {
					log.Infof("worker try to create a new transport...")
					if err := tryAddNewSharedClient(cfg); err != nil {
						log.Warnf(
							"worker: tryAddNewSharedClient: %v",
							err,
						)
					}
				}
			}
		}
	}
}

func tryStartWorker(cfg conf.Values) {
	if workerAlive.Load() {
		return
	}
	go clientsWorker(cfg)
}

func getClient(cfg conf.Values, clientId string) (transport.Transport, error) {
	tryStartWorker(cfg)

	if err := tryAddNewSharedClient(cfg); err != nil {
		return nil, err
	}

	cl := newDividedClient(clientId)
	clients.Store(clientId, cl)

	// log.Debugf("divClient[%s] created.", clientId)

	return cl, nil
}
