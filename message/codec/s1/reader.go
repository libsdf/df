package s1

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	_ "github.com/libsdf/df/log"
	"github.com/libsdf/df/message"
	"io"
)

type Reader struct {
	r io.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

func (r *Reader) ReadArrayHeader() (*ArrayHeader, error) {
	h := make([]byte, 5)
	if _, err := io.ReadFull(r.r, h); err != nil {
		return nil, err
	}
	if h[0] != 'a' {
		return nil, fmt.Errorf("type is not array: %d", h[0])
	}
	size := binary.BigEndian.Uint32(h[1:])
	return &ArrayHeader{Size: int(size)}, nil
}

func (r *Reader) ReadBytes() ([]byte, error) {
	h := make([]byte, 5)
	if _, err := io.ReadFull(r.r, h); err != nil {
		return nil, err
	}
	if h[0] != 'b' {
		return nil, fmt.Errorf("type is not bytes: %d", h[0])
	}
	size := int(binary.BigEndian.Uint32(h[1:]))
	dat := make([]byte, size)
	if _, err := io.ReadFull(r.r, dat); err != nil {
		return nil, err
	}
	return dat, nil
}

func (r *Reader) ReadString() (string, error) {
	h := make([]byte, 5)
	if _, err := io.ReadFull(r.r, h); err != nil {
		return "", err
	}
	if h[0] != 's' {
		return "", fmt.Errorf("type is not bytes: %d", h[0])
	}
	size := int(binary.BigEndian.Uint32(h[1:]))
	dat := make([]byte, size)
	if _, err := io.ReadFull(r.r, dat); err != nil {
		return "", err
	}
	return string(dat), nil
}

func (r *Reader) Read() (message.Message, error) {
	header := make([]byte, 20)
	// log.Debugf("<reader> io.ReadFull(header[20])...")
	if _, err := io.ReadFull(r.r, header); err != nil {
		return nil, err
	}
	kind := header[0]
	id := binary.BigEndian.Uint32(header[8:12])
	ack := binary.BigEndian.Uint32(header[12:16])
	size := int(binary.BigEndian.Uint32(header[16:20]))
	// log.Debugf("<reader> kind=%d, id=%d, size=%d", kind, id, size)

	switch kind {
	case message.KindRequestDNS:
		fallthrough
	case message.KindResultDNS:
		fallthrough
	case message.KindRequestDial:
		fallthrough
	case message.KindConnectionState:
		fallthrough
	case message.KindData:
		fallthrough
	case message.KindPingPong:
		// ok
	default:
		return nil, fmt.Errorf("unknown kind: %d", kind)
	}

	// log.Debugf("<reader> io.ReadFull([%d])...", size)
	dat := make([]byte, size)
	if _, err := io.ReadFull(r.r, dat); err != nil {
		return nil, err
	}

	// log.Debugf("reading message(kind=%d)...", kind)
	switch kind {
	case message.KindRequestDNS:
		m := &message.MessageRequestDNS{}
		if err := json.Unmarshal(dat, m); err != nil {
			return nil, err
		}
		m.Id = id
		// m.Ack = ack
		return m, nil
	case message.KindRequestDial:
		m := &message.MessageRequestDial{}
		if err := json.Unmarshal(dat, m); err != nil {
			return nil, err
		}
		m.Id = id
		// m.Ack = ack
		return m, nil
	case message.KindConnectionState:
		m := &message.MessageConnectionState{}
		if err := json.Unmarshal(dat, m); err != nil {
			return nil, err
		}
		m.Id = id
		m.Ack = ack
		return m, nil
	case message.KindResultDNS:
		r1 := NewReader(bytes.NewReader(dat))
		if arrHeader, err := r1.ReadArrayHeader(); err != nil {
			return nil, err
		} else {
			addrs := [][]byte{}
			for i := 0; i < arrHeader.Size; i += 1 {
				if addrb, err := r1.ReadBytes(); err != nil {
					return nil, err
				} else {
					addrs = append(addrs, addrb)
				}
			}
			errMsg, err := r1.ReadString()
			if err != nil {
				return nil, err
			}
			return &message.MessageResultDNS{
				Id:        id,
				Ack:       ack,
				Addresses: addrs,
				Error:     errMsg,
			}, nil
		}
	case message.KindData:
		r1 := NewReader(bytes.NewReader(dat))
		connId, err := r1.ReadString()
		if err != nil {
			return nil, err
		}
		body, err := r1.ReadBytes()
		if err != nil {
			return nil, err
		}
		return &message.MessageData{
			Id:           id,
			Ack:          ack,
			ConnectionId: connId,
			Data:         body,
		}, nil
	case message.KindPingPong:
		return &message.MessagePing{
			Id:  id,
			Ack: ack,
		}, nil
	}

	return nil, fmt.Errorf("unknown kind: %d", kind)
}
