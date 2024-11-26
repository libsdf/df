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
	"strings"
	"sync/atomic"
	"time"
)

/* type ServerHandler func(string, io.ReadWriteCloser) */

const BUF_SIZE = 1024 * 4

var (
	instances  = new(sync.Map)
	chansReply = new(sync.Map)
)

type replyChannel struct {
	trunkPrefix    string
	ch             chan *Packet
	createdAt      int64
	lastActiveUnix *atomic.Int64
}

type instance struct {
	trunkPrefix string
	clientId    string

	buf *utils.ReadWriteBuffer

	lastActiveUnix *atomic.Int64
	exited         *atomic.Bool
}

func newInstance(trunkPrefix, clientId string) *instance {
	return &instance{
		trunkPrefix:    trunkPrefix,
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

	chReply := (*replyChannel)(nil)
	// lastActiveUnix := int64(0)
	latest := int64(0)
	chansReply.Range(func(k, v interface{}) bool {
		ch := v.(*replyChannel)
		if ch.trunkPrefix != t.trunkPrefix {
			return true
		}
		if latest == 0 || latest < ch.createdAt {
			latest = ch.createdAt
			chReply = ch
		}
		// activeUnix := ch.lastActiveUnix.Load()
		// if lastActiveUnix <= activeUnix {
		// 	lastActiveUnix = activeUnix
		// 	chReply = ch
		// }
		return true
	})

	if chReply == nil {
		// no reply channel
		log.Warnf("no reply channel.")
		return 0, io.EOF
	}

	p := &Packet{
		ClientId: t.clientId,
		Data:     dat,
	}

	chReply.ch <- p
	chReply.lastActiveUnix.Store(time.Now().Unix())

	return len(dat), nil
}

func (t *instance) Close() error {
	return t.buf.Close()
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

	trunkPrefix := ""
	if p := strings.Index(trunkClientId, ":"); p <= 0 {
		tx.Close()
	} else {
		trunkPrefix = trunkClientId[:p]
	}

	log.Infof("[%s] pooled server handler start.", trunkClientId)
	defer log.Infof("[%s] pooled server handler exit.", trunkClientId)

	now := time.Now().Unix()
	chReply := &replyChannel{
		trunkPrefix:    trunkPrefix,
		ch:             make(chan *Packet, 32),
		createdAt:      now,
		lastActiveUnix: new(atomic.Int64),
	}
	chReply.lastActiveUnix.Store(now)
	chansReply.Store(trunkClientId, chReply)

	defer chansReply.Delete(trunkClientId)

	go func() {
		defer cancel()
		for {
			p, err := readPacket(tx)
			if err != nil {
				log.Warnf("readPacket: %v", err)
				return
			}

			if p.Op == OP_PING {
				pong := &Packet{Op: OP_PONG}
				chReply.ch <- pong
				// if err := writePacket(tx, pong); err != nil {
				// 	return
				// }
				continue
			}

			cid := p.ClientId
			if len(cid) == 0 {
				continue
			}

			log.Debugf(
				"[%s] recv packet[%s, %d bytes]",
				trunkPrefix,
				cid,
				len(p.Data),
			)

			if v, found := instances.Load(cid); found {
				inst := v.(*instance)
				inst.writePacket(p)
			} else {
				if p.Serial == 0 {
					inst := newInstance(trunkPrefix, cid)
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
		case p := <-chReply.ch:
			if err := writePacket(tx, p); err != nil {
				log.Warnf("writePacket: %v", err)
				return
			} else {
				chReply.lastActiveUnix.Store(time.Now().Unix())
				log.Debugf(
					"[%s] send packet[%s, %d bytes]",
					trunkPrefix,
					p.ClientId,
					len(p.Data),
				)
			}
		}
	}
}
