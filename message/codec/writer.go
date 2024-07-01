package codec

import (
	"github.com/libsdf/df/message"
)

type Writer interface {
	Write(message.Message) (int, error)
}
