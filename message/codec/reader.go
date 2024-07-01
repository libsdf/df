package codec

import (
	"github.com/libsdf/df/message"
)

type Reader interface {
	Read() (message.Message, error)
}
