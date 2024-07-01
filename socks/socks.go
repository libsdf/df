/*
Package socks contains sub-directories in which implements socks5 client/server and the backends.
*/
package socks

import (
	"bufio"
	"io"
	"net"
)

type Request struct {
	Version  byte
	AddrType byte
	Addr     []byte
	Port     int
}

type Protocol interface {
	Handshake(*bufio.Reader, io.Writer) error
	HandleRequest(*bufio.Reader, net.Conn) error
}
