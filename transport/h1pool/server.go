package h1pool

import (
	"context"
	// "github.com/libsdf/df/transport"
	// "github.com/libsdf/df/transport/h1"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/socks/tunnel"
	"github.com/libsdf/df/utils"
	"io"
	"sync"
	// "bufio"
	"sync/atomic"
	"time"
	// "fmt"
)

/* type ServerHandler func(string, io.ReadWriteCloser) */

const BUF_SIZE = 1024 * 4

var (
	instances = new(sync.Map)
	chReply   = make(chan *Packet, 128)
)

type instance struct {
	clientId string

	buf *utils.ReadWriteBuffer

	lastActiveUnix *atomic.Int64
	exited         *atomic.Bool
}

func newInstance(clientId string) *instance {
	return &instance{
		clientId:       clientId,
		buf:            utils.NewReadWriteBuffer(),
		lastActiveUnix: new(atomic.Int64),
		exited:         new(atomic.Bool),
	}
}

func (t *instance) start() {
	// log.Debugf("instance[%s] started.", t.clientId)
	// defer log.Debugf("instance[%s] exit.", t.clientId)

	defer instances.Delete(t.clientId)
	defer t.exited.Store(true)

	handler := tunnel.NewServerHandler()
	handler(t.clientId, t /* self as tx */)
}

func (t *instance) writePacket(p *Packet) {
	t.buf.Write(p.Data)
}

func (t *instance) Read(buf []byte) (int, error) {
	t.lastActiveUnix.Store(time.Now().Unix())

	return t.buf.Read(buf)
}

func (t *instance) Write(dat []byte) (int, error) {
	t.lastActiveUnix.Store(time.Now().Unix())
	p := &Packet{
		ClientId: t.clientId,
		Data:     dat,
	}
	chReply <- p
	return len(dat), nil
}

func (t *instance) Close() error {
	return nil
}

/*
instancesCleaner removes exited instances from Map.

This cleaner is not really needed.
Instance removes itself when it stops.
*/
func instancesCleaner(x context.Context) {
	for {
		select {
		case <-x.Done():
			return
		case <-time.After(time.Second * 15):
			removingKeys := []interface{}{}
			instances.Range(func(k, value interface{}) bool {
				inst := value.(*instance)
				if inst.exited.Load() {
					removingKeys = append(removingKeys, k)
				}
				return true
			})
			if len(removingKeys) > 0 {
				for _, k := range removingKeys {
					instances.Delete(k)
				}
			}
		}
	}
}

func pooledServerHandler(trunkClientId string, tx io.ReadWriteCloser) {
	// assert(_uselessClientId, "shared")

	// TODO:
	//  (1) read wrapped packet from tx.
	//  (2) wrap reply to packet and sent to tx.

	x, cancel := context.WithCancel(context.Background())

	log.Debugf("pooled server handler start.")
	defer log.Debugf("pooled server handler exit.")

	go instancesCleaner(x)

	// r := bufio.NewReader(tx)

	// // check the packet header magic
	// if d, err := r.Peek(3); err != nil {
	//     h := tunnel.NewServerHandler()
	//     h(trunkClientId,
	//     return
	// }

	go func() {
		defer cancel()
		for {
			p, err := readPacket(tx)
			if err != nil {
				log.Warnf("readPacket: %v", err)
				return
			}
			cid := p.ClientId
			if len(cid) == 0 {
				continue
			}

			// log.Debugf("recv packet[%s, %d bytes]", cid, len(p.Data))

			if v, found := instances.Load(cid); found {
				inst := v.(*instance)
				inst.writePacket(p)
			} else {
				if p.Serial == 0 {
					inst := newInstance(cid)
					instances.Store(cid, inst)
					go inst.start()
					inst.writePacket(p)
				}
			}
		}
	}()

	for {
		select {
		case <-x.Done():
			return
		case p := <-chReply:
			if err := writePacket(tx, p); err != nil {
				log.Warnf("writePacket: %v", err)
				return
			} else {
				// log.Debugf("send packet[%s, %d bytes]",
				//     p.ClientId, len(p.Data),
				// )
			}
		}
	}
}
