package utils

import (
	"io"
	"net"
)

func IsIOError(err error) bool {
	if err != nil {
		if err == io.EOF {
			return true
		}
		switch err.(type) {
		case *net.OpError:
			return true
		}
	}
	return false
}
