package tunnel

import (
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/message"
	"github.com/libsdf/df/message/codec/s1"
	"github.com/libsdf/df/utils"
	"io"
	"net"
	"sync"
)

type ServerHandler func(string, io.ReadWriteCloser)

var (
	clients = new(sync.Map)
)

func lookupIP(host string) ([]net.IP, error) {
	if v, found := dnsCache.Get(host); found {
		if ipAddr, ok := v.([]net.IP); ok {
			return ipAddr, nil
		}
	}
	if ipAddr, err := net.LookupIP(host); err != nil {
		return nil, err
	} else {
		dnsCache.Put(host, ipAddr)
		return ipAddr, nil
	}
}

type clientSession struct {
	id   string
	conn io.ReadWriteCloser
}

func (s *clientSession) debug(line string, pars ...interface{}) {
	// log.Debugf("<cid:%s> %s", s.id, fmt.Sprintf(line, pars...))
}

func (s *clientSession) handleDialConn(m1 *message.MessageRequestDial, r *s1.Reader, w *s1.Writer) error {
	connId := m1.ConnectionId

	debug := func(line string, pars ...interface{}) {
		// log.Debugf("<conn:%s> %s", connId, fmt.Sprintf(line, pars...))
	}

	host, portStr, err := net.SplitHostPort(m1.Address)
	if err != nil {
		return err
	}

	hostTarget := m1.Address

	if ip := net.ParseIP(host); ip != nil {
		if !utils.IsLegitProxiableIP(ip) {
			debug("refused the dial request to ip %s", ip)
			return fmt.Errorf("invalid target ip: %s", ip)
		}
		hostTarget = ip.String()
	} else {
		if ipAddrs, err := lookupIP(host); err != nil {
			debug("unable to resolve host `%s`: %v", host, err)
			return fmt.Errorf("unable to resolve %s", host)
		} else {
			if len(ipAddrs) <= 0 {
				return fmt.Errorf("unable to resolve %s", host)
			}
			ip := net.IP(nil)
			for _, v := range ipAddrs {
				if len(v) == 4 {
					ip = v
					break
				}
			}
			if !utils.IsLegitProxiableIP(ip) {
				return fmt.Errorf("not a legit ip: %s", ip)
			}
			hostTarget = ip.String()
		}
	}

	addrTarget := fmt.Sprintf("%s:%s", hostTarget, portStr)
	debug("connecting to %s -> %s ...", m1.Address, addrTarget)

	conn, err := net.Dial(m1.Network, addrTarget)
	if err != nil {
		reply := &message.MessageConnectionState{
			ConnectionId: connId,
			State:        "error",
			Error:        fmt.Sprintf("net.Dial: %v", err),
		}
		if _, err := w.Write(reply); err != nil {
			return err
		}
		return err
	}
	defer conn.Close()
	debug("connected to %s", m1.Address)

	m := &message.MessageConnectionState{
		ConnectionId: connId,
		State:        "open",
	}
	if _, err := w.Write(m); err != nil {
		log.Warnf("w.Write(connectionState<open>): %v", err)
		return err
	}

	chConn := make(chan *message.MessageConnectionState)
	chDat := make(chan *message.MessageData)
	chErr := make(chan error, 2)

	chDown := make(chan []byte) // data from target server to client.

	go func() {
		// reading from server
		maxBufSize := conf.MAX_BUFFER_SIZE
		bufSize := conf.DEFAULT_BUFFER_SIZE
		buf := make([]byte, bufSize)
		countFullRead := 0
		for {
			if size, err := conn.Read(buf); err != nil {
				chErr <- err
				return
			} else {
				chunk := make([]byte, size)
				copy(chunk, buf[:size])
				chDown <- chunk

				// check if buf should be enlarged.
				if size == bufSize {
					countFullRead += 1
					if countFullRead >= 10 && bufSize < maxBufSize {
						bufSize *= 2
						buf = make([]byte, bufSize)
						// log.Debugf("buffer size set to %d", bufSize)
					}
				} else {
					countFullRead = 0
				}

			}
		}
	}()

	go func() {
		// reading from client.
		for {
			m, err := r.Read()
			if err != nil {
				chErr <- err
				return
			}
			switch m1 := m.(type) {
			case *message.MessageData:
				chDat <- m1
			case *message.MessageConnectionState:
				chConn <- m1
			}
		}
	}()

	for {
		select {
		case connState := <-chConn:
			// control instruction from client.
			if connState.State == "close" {
				return nil
			}
		case mDat := <-chDat:
			// data from client
			// debug("data from client: %d bytes", len(mDat.Data))
			if _, err := conn.Write(mDat.Data); err != nil {
				return err
			}
		case dat := <-chDown:
			// data to client
			// debug("data to client: %d bytes", len(dat))
			mDat := &message.MessageData{
				ConnectionId: connId,
				Data:         dat,
			}
			if _, err := w.Write(mDat); err != nil {
				return err
			}
		case err := <-chErr:
			// debug("chErr: %v", err)
			return err
		}
	}

	return nil
}

func (s *clientSession) handleDNS(m1 *message.MessageRequestDNS, r *s1.Reader, w *s1.Writer) error {
	reply := &message.MessageResultDNS{}
	reply.Ack = m1.MessageId()

	s.debug("<nsreq:%d> resolving `%s`...", m1.Id, m1.DomainName)
	ips, err := net.LookupIP(m1.DomainName)
	if err != nil {
		reply.Error = fmt.Sprintf("%v", err)
	} else {
		reply.Addresses = make([][]byte, len(ips))
		for i, ipBytes := range ips {
			reply.Addresses[i] = []byte(ipBytes)
		}
	}
	s.debug("<nsreq:%d> write reply(ack=%d): %v", m1.Id, reply.Ack, reply)
	if _, err := w.Write(reply); err != nil {
		log.Warnf("<nsreq:%d> <cid:%s> w.Write: %v", m1.Id, s.id, err)
		return err
	}
	return nil
}

func (s *clientSession) handle() {
	r := s1.NewReader(s.conn)
	w := s1.NewWriter(s.conn)
	serial := 0
	for {
		// s.debug("reading the #%d message...", serial)
		serial += 1
		if m, err := r.Read(); err != nil {
			log.Errorf("<cid:%s> r.Read: %v", s.id, err)
			return
		} else {
			kind := m.Kind()
			s.debug("request received, kind=%d", kind)

			switch kind {
			case message.KindRequestDNS:
				m1 := m.(*message.MessageRequestDNS)
				if err := s.handleDNS(m1, r, w); err != nil {
					return
				}
			case message.KindRequestDial:
				m1 := m.(*message.MessageRequestDial)
				if err := s.handleDialConn(m1, r, w); err != nil {
					return
				}
				return
			default:
				log.Warnf("<cid:%s> message(kind=%d) ignore.",
					s.id, kind)
			}
		}
	}
}

type server struct {
}

func (s *server) handleSession(clientId string, conn io.ReadWriteCloser) {
	// log.Debugf("handleSession: clientId=%s", clientId)
	sess := &clientSession{
		id:   clientId,
		conn: conn,
	}
	sess.handle()
}

func NewServerHandler() ServerHandler {
	s := &server{}
	return s.handleSession
}
