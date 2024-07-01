package backend

import (
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/message"
	"github.com/libsdf/df/message/codec/s1"
	"github.com/libsdf/df/transport"
	_ "github.com/libsdf/df/transport/d1"
	_ "github.com/libsdf/df/transport/h1"
	"github.com/libsdf/df/utils"
	"net"
	"sync"
	"time"
)

var (
	seedNsReqId uint32 = 0
	lck                = new(sync.Mutex)
)

var (
	mapNsWait = new(sync.Map)
)

func nextNsReqId() uint32 {
	lck.Lock()
	defer lck.Unlock()
	if seedNsReqId > (1 << 15) {
		seedNsReqId = 0
	}
	seedNsReqId += 1
	return seedNsReqId
}

type messageHandler func(message.Message)

type addrsHandler func([]net.IP)

type uplinkSecure struct {
	id string
	tx transport.Transport

	msgId uint32
	lck   *sync.Mutex

	mapWait     *sync.Map
	mapWaitConn *sync.Map

	// mapNsWait *sync.Map
}

func (l *uplinkSecure) nextMsgId() uint32 {
	l.lck.Lock()
	defer l.lck.Unlock()
	l.msgId += 1
	if l.msgId >= uint32(1)<<15 {
		l.msgId = 1
	}
	return l.msgId
}

func (l *uplinkSecure) debug(line string, pars ...interface{}) {
	// msg := fmt.Sprintf(line, pars...)
	// log.Debugf("<cid:%s> %s", l.id, msg)
}

func (l *uplinkSecure) LookupIP(network, dn string) ([]net.IP, error) {
	if v, found := dnsCache.Get(dn); found {
		switch addrs := v.(type) {
		case []net.IP:
			return addrs, nil
		}
	}

	nsReqId := nextNsReqId()

	if v, found := mapNsWait.LoadOrStore(dn, new(sync.Map)); found {
		// log.Debugf("resolving `%s`: already started. add a callback.", dn)
		ch := make(chan []net.IP)
		onLoad := addrsHandler(func(addrs []net.IP) {
			ch <- addrs
		})
		if mapCallbacks, ok := v.(*sync.Map); ok {
			mapCallbacks.Store(nsReqId, onLoad)
		}
		select {
		case addrs := <-ch:
			// log.Debugf("resolving `%s`: callback invoked.", dn)
			return addrs, nil
		case <-time.After(time.Second * 5):
			l.debug("resolving `%s`: callback waiting timeout.", dn)
			break
		}
	}

	defer mapNsWait.Delete(dn)

	msgId := nsReqId

	req := &message.MessageRequestDNS{
		Id:         nsReqId,
		Network:    network,
		DomainName: dn,
	}

	ch := make(chan *message.MessageResultDNS)

	fn := messageHandler(func(m message.Message) {
		l.debug("received reply: ack=%d, kind=%v",
			m.AckMessageId(), m.Kind())
		switch rs := m.(type) {
		case *message.MessageResultDNS:
			ch <- rs
			return
		}
		ch <- nil
	})
	l.mapWait.Store(msgId, fn)

	l.debug("<nsreq:%d> resolving `%s`, msgId=%d ...", nsReqId, dn, msgId)
	w := s1.NewWriter(l.tx)
	if _, err := w.Write(req); err != nil {
		if !utils.IsIOError(err) {
			log.Warnf("w.Write: %v", err)
		}
		return nil, err
	}

	durTotal := time.Duration(0)

	for {
		select {
		case <-time.After(time.Second * 5):
			// log.Debugf("waiting nslookup response...")
			durTotal += time.Second * 5
		case rs := <-ch:
			l.debug("<nsreq:%d> resolving result received: %v", nsReqId, rs)
			if rs != nil {
				ipList := make([]net.IP, len(rs.Addresses))
				for i, addr := range rs.Addresses {
					ipList[i] = net.IP(addr)
				}

				// save to cache.
				dnsCache.Put(dn, ipList)

				// invoke callbacks
				if v, found := mapNsWait.Load(dn); found {
					if mapCallbacks, ok := v.(*sync.Map); ok {
						mapCallbacks.Range(func(_, v interface{}) bool {
							if h, ok := v.(addrsHandler); ok {
								h(ipList)
							}
							return true
						})
					}
				}

				return ipList, nil
			}
		}
		if durTotal > time.Second*20 {
			break
		}
	}

	// log.Warnf("<cid:%s> failed resolving at remote. switch to local resolver.", l.id)
	return nil, fmt.Errorf("<cid:%s> resolving timeout: %s", l.id, dn)
	// return net.LookupIP(dn)
}

func (l *uplinkSecure) Dial(network, addr string) (Conn, error) {
	connId := UniqueId(8)

	req := &message.MessageRequestDial{
		ConnectionId: connId,
		Network:      network,
		Address:      addr,
	}

	chDown := make(chan []byte)
	chUp := make(chan []byte)

	chErr := make(chan error)
	chState := make(chan *message.MessageConnectionState)

	fn := messageHandler(func(m message.Message) {
		l.debug("received message: kind=%v", m.Kind())
		switch m1 := m.(type) {
		case *message.MessageConnectionState:
			chState <- m1
		case *message.MessageData:
			chDown <- m1.Data
		}
	})
	// log.Infof("watching connId=%s", connId)
	l.mapWaitConn.Store(connId, fn)

	w := s1.NewWriter(l.tx)
	if _, err := w.Write(req); err != nil {
		if !utils.IsIOError(err) {
			log.Warnf("w.Write: %v", err)
		}
		return nil, err
	}

	select {
	case <-time.After(time.Second * 30):
		return nil, fmt.Errorf("failed to connect: timeout.")
	case m1 := <-chState:
		switch m1.State {
		case "open":
			// ok
		default:
			return nil, fmt.Errorf("failed to connect: %s", m1.Error)
		}
	}

	conn := &virtualConn{
		lck:    new(sync.RWMutex),
		closed: false,

		chR:   chDown,
		chW:   chUp,
		chErr: chErr,

		bufRead:    make([]byte, 1024*16),
		lenBufRead: 0,
	}

	go func() {
		defer conn.closeLocal()
		defer l.mapWaitConn.Delete(connId)

		for {
			select {
			case dat := <-chUp:
				// client to server
				m := &message.MessageData{
					ConnectionId: connId,
					Data:         dat,
				}
				if _, err := w.Write(m); err != nil {
					if !utils.IsIOError(err) {
						log.Warnf("w.Write(kind=%d): %v", m.Kind(), err)
					}
					return
				} else {
					// log.Debugf("<conn:%s> data sent: %d bytes",
					// 	connId, len(dat),
					// )
				}

			case err := <-chErr:
				if err != nil {
					if !utils.IsIOError(err) {
						log.Warnf("%v", err)
					}
				}
				return
			}
		}
	}()

	return conn, nil
	// return net.Dial(network, addr)
}

func (l *uplinkSecure) dispatchReply(m message.Message) {
	ack := m.AckMessageId()
	if v, found := l.mapWait.LoadAndDelete(ack); found {
		// log.Infof("reply waiter for ack=%d: found %T", ack, v)
		switch fn := v.(type) {
		case messageHandler:
			fn(m)
		default:
			log.Warnf("not a message handler!")
		}
	} else {
		log.Warnf("reply waiter for ack=%d: not found.", ack)
	}
}

func (l *uplinkSecure) dispatchConnMsg(m message.Message) {
	connId := ""
	switch m1 := m.(type) {
	case *message.MessageConnectionState:
		connId = m1.ConnectionId
		l.debug("<conn:%s> state received: %s", connId, m1.State)
	case *message.MessageData:
		connId = m1.ConnectionId
		l.debug("<conn:%s> data received: %d bytes",
			connId, len(m1.Data),
		)
	}
	if len(connId) > 0 {
		if v, found := l.mapWaitConn.Load(connId); found {
			fn := v.(messageHandler)
			fn(m)
		}
	}
}

func (l *uplinkSecure) worker() {
	// defer log.Warnf("worker exit.")
	r := s1.NewReader(l.tx)
	for {
		if m, err := r.Read(); err != nil {
			if !utils.IsIOError(err) {
				log.Warnf("<cid:%s> r.Read: %v", l.id, err)
			}
			return
		} else {
			if m.AckMessageId() > 0 {
				l.dispatchReply(m)
			} else {
				l.dispatchConnMsg(m)
			}
		}
	}
}

func (l *uplinkSecure) Close() {
	l.tx.Close()
}

func BackendSecure(suitName string, params conf.Values) Backend {
	id := UniqueId(16)
	// log.Infof("create secure tunnel: id=%s", id)

	protoSuit := transport.GetSuit(suitName)
	if protoSuit == nil {
		panic(fmt.Errorf("protocol suit not found: %s", suitName))
	}

	pars := params.Clone()
	cl, err := protoSuit.Client(pars, id)
	if err != nil {
		panic(err)
	}

	l := &uplinkSecure{
		id:          id,
		tx:          cl,
		mapWait:     new(sync.Map),
		mapWaitConn: new(sync.Map),
		lck:         new(sync.Mutex),
	}
	go l.worker()

	return l
}
