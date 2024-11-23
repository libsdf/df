package socks5

import (
	"encoding/binary"
	"fmt"
	"github.com/libsdf/df/log"
	"io"
	"net"
	"strconv"
)

// Socks5Connect makes a transport to target address indicated by addr through the proxy server indicated by proxyAddr.
func Socks5Connect(proxyAddr, addr string) (io.ReadWriteCloser, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	// connect to socks5 proxy server
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		log.Warnf("net.Dial(%s): %v", proxyAddr, err)
		return nil, err
	}

	// initiate handshaking
	hs := []byte{5, 0}
	if _, err := conn.Write(hs); err != nil {
		log.Warnf("conn.Write(hs): %v", err)
		return nil, err
	}

	hsAck := make([]byte, 2)
	if _, err := io.ReadFull(conn, hsAck); err != nil {
		log.Warnf("io.ReadFull(hsAck): %v", err)
		return nil, err
	}

	if hsAck[0] != 5 || hsAck[1] != 0 {
		return nil, fmt.Errorf("socks5 handshake failed.")
	}

	addrTyp := byte(1)
	ip := net.ParseIP(host)
	if len(ip) == 4 {
		/* 1: ipv4 */
		addrTyp = byte(1)
	} else if len(ip) == 16 {
		/* 4: ipv6 */
		addrTyp = byte(4)
	} else {
		/* 3: domain name */
		addrTyp = byte(3)
	}

	lenS5Req := 4 + len(ip)
	if ip == nil {
		lenS5Req = 4 + 1 + len(host)
	}
	lenS5Req += 2 /* for port, uint16 */

	/* leading 4 bytes: [version=5, command=1, null=0, addrTyp=?] */
	s5req := make([]byte, lenS5Req)
	s5req[0] = 5
	s5req[1] = 1 /* make tcp conn */
	s5req[3] = addrTyp

	if ip != nil {
		copy(s5req[4:], ip)
	} else {
		s5req[4] = byte(len(host))
		copy(s5req[5:], []byte(host))
	}

	// put port into last 2 bytes
	portb := make([]byte, 2)
	binary.BigEndian.PutUint16(portb, uint16(port))
	copy(s5req[len(s5req)-2:], portb)

	// send socks5 request
	if _, err := conn.Write(s5req); err != nil {
		log.Warnf("conn.Write(s5req): %v", err)
		return nil, err
	}

	s5reqAck := make([]byte, 10)
	if _, err := io.ReadFull(conn, s5reqAck); err != nil {
		log.Warnf("io.ReadFull(s5reqAck): %v", err)
		return nil, err
	}
	if s5reqAck[0] != 5 {
		return nil, fmt.Errorf("wrong version.")
	} else if s5reqAck[3] != 1 {
		return nil, fmt.Errorf("server side failed.")
	}

	// transport established.

	return conn, nil
}
