package backend

import (
	"github.com/libsdf/df/conf"
	"io"
	"net"
)

type Conn = io.ReadWriteCloser

type Resolver interface {
	LookupIP(string, string) ([]net.IP, error)
}

type Dialer interface {
	Dial(string, string) (Conn, error)
}

type Backend interface {
	Resolver
	Dialer
	Close()
}

type BackendProvider func() Backend

// var (
// 	linker = (Linker)(nil)
// )

func GetBackend(suitName string, params conf.Values) Backend {
	if len(suitName) > 0 {
		// if linker == nil {
		// 	linker = SecureTunnel(suitName, params)
		// }
		// return linker
		return BackendSecure(suitName, params)
	}
	return Direct()
}
