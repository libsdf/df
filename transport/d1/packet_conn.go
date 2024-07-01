package d1

import (
	"net"
	"time"
)

type packetConn struct {
	createdAt      time.Time
	lastActiveTime time.Time
	remoteAddr     *net.UDPAddr
	chR            chan []byte
}
