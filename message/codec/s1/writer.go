package s1

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/libsdf/df/message"
	"io"
)

type Writer struct {
	w io.Writer
}

func (w *Writer) writeMany(chunks ...[]byte) (int, error) {
	if len(chunks) <= 0 {
		return 0, nil
	}
	total := 0
	for _, chunk := range chunks {
		if len(chunk) <= 0 {
			continue
		}
		total += len(chunk)
	}
	chunkAll := make([]byte, total)
	i := 0
	for _, chunk := range chunks {
		copy(chunkAll[i:], chunk)
		i += len(chunk)
	}
	return w.w.Write(chunkAll)
}

func (w *Writer) WriteBytes(dat []byte) (int, error) {
	chunk := make([]byte, 5+len(dat))
	chunk[0] = 'b'
	binary.BigEndian.PutUint32(chunk[1:5], uint32(len(dat)))
	copy(chunk[5:], dat)
	return w.w.Write(chunk)
}

func (w *Writer) WriteArrayHeader(size int) (int, error) {
	header := []byte{'a', 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[1:], uint32(size))
	return w.w.Write(header)
}

func (w *Writer) WriteString(s string) (int, error) {
	sbytes := []byte(s)
	dat := make([]byte, 5+len(sbytes))
	size := len(sbytes)
	dat[0] = 's'
	binary.BigEndian.PutUint32(dat[1:5], uint32(size))
	copy(dat[5:], sbytes)
	return w.w.Write(dat)
}

func (w *Writer) WriteInt32(i int32) (int, error) {
	dat := make([]byte, 5)
	dat[0] = 'i'
	binary.BigEndian.PutUint32(dat[1:], uint32(i))
	return w.w.Write(dat)
}

func (w *Writer) WriteInt64(i int64) (int, error) {
	dat := make([]byte, 9)
	dat[0] = 'I'
	binary.BigEndian.PutUint64(dat[1:], uint64(i))
	return w.w.Write(dat)
}

func (w *Writer) WriteBool(v bool) (int, error) {
	dat := make([]byte, 2)
	dat[0] = '='
	if v {
		dat[1] = 'y'
	} else {
		dat[1] = 'n'
	}
	return w.w.Write(dat)
}

func (w *Writer) Write(m message.Message) (int, error) {
	kind := m.Kind()

	/* kind(1) + reserved(7) + id(4) + ack(4) + len(4) */
	header := make([]byte, 20)
	header[0] = kind

	binary.BigEndian.PutUint32(header[8:12], m.MessageId())
	binary.BigEndian.PutUint32(header[12:16], m.AckMessageId())

	switch kind {
	case message.KindRequestDNS:
		fallthrough
	case message.KindRequestDial:
		fallthrough
	case message.KindConnectionState:
		if dat, err := json.Marshal(m); err != nil {
			return 0, err
		} else {
			size := uint32(len(dat))
			binary.BigEndian.PutUint32(header[16:], size)
			return w.writeMany(header, dat)
		}
	case message.KindResultDNS:
		m1 := m.(*message.MessageResultDNS)
		buf := bytes.NewBuffer([]byte{})
		w1 := &Writer{w: buf}
		arrLen := len(m1.Addresses)
		if _, err := w1.WriteArrayHeader(arrLen); err != nil {
			return 0, err
		}
		for _, addr := range m1.Addresses {
			if _, err := w1.WriteBytes(addr); err != nil {
				return 0, err
			}
		}
		if _, err := w1.WriteString(m1.Error); err != nil {
			return 0, err
		}
		dat := buf.Bytes()
		size := uint32(len(dat))
		binary.BigEndian.PutUint32(header[16:], uint32(size))
		return w.writeMany(header, dat)
	case message.KindData:
		m1 := m.(*message.MessageData)
		buf := bytes.NewBuffer([]byte{})
		w1 := NewWriter(buf)
		total := 0
		if size, err := w1.WriteString(m1.ConnectionId); err != nil {
			return 0, err
		} else {
			total += size
		}
		if size, err := w1.WriteBytes(m1.Data); err != nil {
			return 0, err
		} else {
			total += size
		}
		binary.BigEndian.PutUint32(header[16:], uint32(total))
		return w.writeMany(header, buf.Bytes())
	case message.KindPingPong:
		buf := make([]byte, 8)
		rand.Read(buf)
		binary.BigEndian.PutUint32(header[16:], uint32(8))
		return w.writeMany(header, buf)
	}
	return 0, fmt.Errorf("unknown kind: %d", kind)
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}
