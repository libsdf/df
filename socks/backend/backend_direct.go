package backend

import (
	"net"
)

type uplinkDirect struct{}

func (l *uplinkDirect) LookupIP(network, dn string) ([]net.IP, error) {
	return net.LookupIP(dn)
}

func (l *uplinkDirect) Dial(network, addr string) (Conn, error) {
	return net.Dial(network, addr)
}

func (l *uplinkDirect) Close() {
}

func Direct() Backend {
	l := &uplinkDirect{}
	return l
}
