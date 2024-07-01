/*
Package transport contains implementations of kinds of transports.
There are server and client roles in a [TransportSuit].
A kind of transport should implements the interface [TransportSuit], and call [Register] method to register itself.
The implementations can be found in sub-directions like h1, d1.
*/
package transport

import (
	"github.com/libsdf/df/conf"
	"io"
	"sync"
)

type Transport = io.ReadWriteCloser

type TransportSuit interface {
	Name() string
	Enabled() bool
	Server(conf.Values) error
	Client(conf.Values, string) (Transport, error)
}

var reg = new(sync.Map)

func Register(suit TransportSuit) {
	name := suit.Name()
	reg.Store(name, suit)
}

func GetSuit(name string) TransportSuit {
	if v, found := reg.Load(name); found {
		return v.(TransportSuit)
	}
	return nil
}
