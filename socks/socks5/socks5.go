/*
Package socks5 implements a socks5 server and a client connector method.

# Example

	import (
	    "context"
	    "github.com/libsdf/df/conf"
	)

	func main() {
	    params := make(conf.Values)
	    params.Set(conf.FRAMER_PSK, "******") // encryption AES key
	    params.Set(conf.SERVER_URL, "http://localhost:1081") // a transport server runs at port 1081.
	    options := &Options{
	        Port: 1080,
	        ProtocolSuit: "h1",
	        ProtocolParams: params,
	    }
	    Server(context.Background(), options)
	}
*/
package socks5

import (
	"bufio"
	"fmt"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/socks/backend"
	"github.com/libsdf/df/utils"
	"io"
	"net"
)

var (
	backendDirect = backend.Direct()
)

type v5 struct {
	options *Options
}

func newV5(options *Options) *v5 {
	return &v5{
		options: options,
	}
}

func (s *v5) Handshake(r *bufio.Reader, w io.Writer) error {
	ver, err := r.ReadByte()
	if err != nil {
		return err
	}
	if ver != 5 {
		return fmt.Errorf("wrong version.")
	}

	nauth, err := r.ReadByte()
	if err != nil {
		return err
	}

	if nauth > 0 {
		for i := 0; i < int(nauth); i += 1 {

			auth, err := r.ReadByte()
			if err != nil {
				return err
			}

			switch auth {
			case 0:
				// No authentication
			case 1:
				// GSSAPI
			case 2:
				// Username/password, RFC1929
			case 3:
				// Challenge-handshake
			case 4:
				// Unassigned
			case 5:
				// Challenge-response
			case 6:
				// SSL
			case 7:
				// NDS
			case 8:
				// multi-authentication framework
			case 9:
				// JSON parameter block
			default:
				// Unassigned or reserved
			}
		}
	}

	// todo: authentication
	w.Write([]byte{ver, 0})

	return nil
}

func (s *v5) HandleRequest(r *bufio.Reader, w net.Conn) error {
	// sessionId := fmt.Sprintf("%s", w.RemoteAddr())
	// log.Infof("<%s> socks5 start..", sessionId)
	// defer log.Infof("<%s> socks5 bye.", sessionId)

	ver, err := r.ReadByte()
	if err != nil {
		if !utils.IsIOError(err) {
			log.Errorf("%v", err)
		}
		return err
	}
	if ver != 5 {
		return fmt.Errorf("incompatible version.")
	}

	cmd, err := r.ReadByte()
	if err != nil {
		log.Errorf("%v", err)
		return err
	}
	switch cmd {
	case 1:
		// make TCP/IP connection.
	default:
		log.Errorf("unsupported cmd: %d", cmd)
		w.Write([]byte{ver, 7, 0, 0, 0, 0})
		return fmt.Errorf("unsupported cmd: %d", cmd)
	}

	_, err = r.ReadByte()
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	addrTyp, err := r.ReadByte()
	if err != nil {
		log.Errorf("%v", err)
		return err
	}
	// log.Debugf("addrTyp: %d", addrTyp)

	// log.Debugf("<%s> socks5 request initialized.", sessionId)

	// linker := uplink.Direct()
	// linker := (uplink.Linker)(nil)
	// linker := backend.GetBackend(
	// 	s.options.ProtocolSuit,
	// 	s.options.ProtocolParams,
	// )
	// defer linker.Close()

	hostDst := ""
	isHostLocal := false
	// ipListDst := []net.IP{}

	switch addrTyp {
	case 1:
		// ipv4: 4 bytes
		ipBytes := make([]byte, 4)
		if _, err := io.ReadFull(r, ipBytes); err != nil {
			log.Errorf("%v", err)
			return err
		}
		ip := net.IP(ipBytes)
		hostDst = ip.String()
		isHostLocal = !utils.IsLegitProxiableIP(ip)
		// ipListDst = append(ipListDst, net.IP(ip))
	case 3:
		// domain name
		sizeb, err := r.ReadByte()
		if err != nil {
			log.Errorf("%v", err)
			return err
		}
		size := int(sizeb)
		nameb := make([]byte, size)
		if _, err := io.ReadFull(r, nameb); err != nil {
			log.Errorf("%v", err)
			return err
		}

		hostDst = string(nameb)
		// dn := string(nameb)

		// addrs, err := linker.LookupIP("ip", dn)
		// if err != nil {
		// 	log.Errorf("net.LookupHost: %v", err)
		// 	return err
		// }
		// if len(addrs) > 0 {
		// 	// log.Debugf("<%s> resolved `%s` to %s", sessionId, dn, addrs[0])
		// }
		// ipListDst = append(ipListDst, addrs...)

	case 4:
		// ipv6: 16 bytes
		ipBytes := make([]byte, 16)
		if _, err := io.ReadFull(r, ipBytes); err != nil {
			log.Errorf("%v", err)
			return err
		}
		ip := net.IP(ipBytes)
		hostDst = net.IP(ipBytes).String()
		isHostLocal = !utils.IsLegitProxiableIP(ip)
		// ipListDst = append(ipListDst, net.IP(ip6))
	}

	portb := make([]byte, 2)
	if _, err := io.ReadFull(r, portb); err != nil {
		log.Errorf("%v", err)
		return err
	}

	portDst := int(portb[0])*256 + int(portb[1])

	netDst := "tcp"
	// ipDstStr := ""
	// for _, ip := range ipListDst {
	// 	if len(ip) == 4 {
	// 		ipDstStr = ip.String()
	// 		break
	// 	}
	// }
	// if len(ipDstStr) <= 0 && len(ipListDst) > 0 {
	// 	ipDstStr = ipListDst[0].String()
	// }
	// if len(ipDstStr) <= 0 {
	// 	w.Write([]byte{ver, 4, 0, 0, 0, 0})
	// 	log.Errorf("unable to resolve name.")
	// 	return fmt.Errorf("unable to resolve name.")
	// }

	// addrDst := fmt.Sprintf("%s:%d", ipDstStr, portDst)
	addrDst := fmt.Sprintf("%s:%d", hostDst, portDst)

	linker := (backend.Backend)(nil)
	if s.options.BackendProvider != nil {
		linker = s.options.BackendProvider()
	}
	if linker == nil {
		if !isHostLocal {
			linker = backend.GetBackend(
				s.options.ProtocolSuit,
				s.options.ProtocolParams,
			)
		} else {
			linker = backendDirect
		}
	}
	defer linker.Close()

	// log.Debugf("<%s> connecting to %s", sessionId, addrDst)
	upConn, err := linker.Dial(netDst, addrDst)
	if err != nil {
		log.Errorf("backend.Dial(%s, %s): %v", netDst, addrDst, err)
		w.Write([]byte{ver, 4, 0, 0, 0, 0})
		return err
	}
	// log.Debugf("<%s> connected to %s.", sessionId, addrDst)

	defer upConn.Close()
	// defer log.Debugf("<%s> disconnected from %s.", sessionId, addrDst)

	_, err = w.Write([]byte{ver, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	if err != nil {
		return err
	}

	// log.Debugf("<%s> do proxy ...", sessionId)
	utils.Proxy(w, upConn)

	return nil
}
