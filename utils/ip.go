package utils

import (
	"net"
)

func IsLegitProxiableIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLinkLocalUnicast() {
		return false
	} else if ip.IsLinkLocalMulticast() {
		return false
	} else if ip.IsLoopback() {
		return false
	} else if ip.IsPrivate() {
		return false
	} else if ip.IsMulticast() {
		return false
	} else if ip.IsUnspecified() {
		return false
	}
	return true
}
