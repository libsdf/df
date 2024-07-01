/*
Package framer holds packages that pack traffic chunks to encrypted frames.
*/
package framer

import (
	"io"
)

type Framer interface {
	io.ReadWriteCloser
}
