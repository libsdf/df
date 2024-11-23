package h1pool

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

var magic = []byte{'C', 'P', 0}

const MAX_PACKET_SIZE = 1024 * 128
const MAX_META_SIZE = 1024

const (
	OP_DATA = 0
	OP_PING = 8
)

/*
packet structure:

| magic:       3 bytes | op: 1 byte |---- \
| serial                    4 bytes |       header part
| meta length:              4 bytes |     /
| data length:              4 bytes |----/
| meta ...............        ..... |
| data ...............              |
| ...                         ..... |

*/

type Packet struct {
	ClientId string `json:"client_id"`
	Serial   uint32 `json:"-"`
	Op       uint8  `json:"-"`
	Data     []byte `json:"-"`
}

func readAnyPacket(r io.Reader) (*Packet, error) {
	head := make([]byte, 16)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, err
	}
	if !bytes.Equal(magic, head[:3]) {
		return nil, fmt.Errorf("illegal packet header.")
	}

	op := head[3]

	switch op {
	case OP_DATA, OP_PING:
		break
	default:
		return nil, fmt.Errorf("Invalid op.")
	}

	serial := binary.BigEndian.Uint32(head[4:8])

	sizeMeta := int(binary.BigEndian.Uint32(head[8:12]))
	if sizeMeta < 0 {
		return nil, fmt.Errorf("Invalid meta size field.")
	} else if sizeMeta > MAX_META_SIZE {
		return nil, fmt.Errorf("Meta size too large.")
	}

	sizeDat := int(binary.BigEndian.Uint32(head[12:16]))
	if sizeDat < 0 {
		return nil, fmt.Errorf("Invalid packet data size field.")
	} else if sizeDat > MAX_PACKET_SIZE {
		return nil, fmt.Errorf("Packet data size too large.")
	}

	p := &Packet{Op: op, Serial: serial}

	if sizeMeta > 0 {
		metab := make([]byte, sizeMeta)
		if _, err := io.ReadFull(r, metab); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metab, p); err != nil {
			return nil, err
		}
	}

	if sizeDat > 0 {
		dat := make([]byte, sizeDat)
		if _, err := io.ReadFull(r, dat); err != nil {
			return nil, err
		}
		p.Data = dat
	}

	return p, nil
}

// readPacket reads next data packet, ignoring ping packet.
func readPacket(r io.Reader) (*Packet, error) {
	for {
		if p, err := readAnyPacket(r); err != nil {
			return nil, err
		} else if p.Op == OP_PING {
			continue
		} else {
			return p, nil
		}
	}
}

func writePacket(w io.Writer, p *Packet) error {
	metab := ([]byte)(nil)
	if p.Op == OP_DATA {
		if d, err := json.Marshal(p); err != nil {
			return err
		} else {
			metab = d
		}
	}

	start := 0
	for {
		sizeDat := len(p.Data) - start
		if sizeDat > MAX_PACKET_SIZE {
			sizeDat = MAX_PACKET_SIZE
		}
		dat := ([]byte)(nil)
		if sizeDat > 0 {
			dat = p.Data[start : start+sizeDat]
		}

		size := 16
		head := make([]byte, size)
		copy(head[:3], magic)
		head[3] = p.Op
		binary.BigEndian.PutUint32(head[4:8], p.Serial)
		binary.BigEndian.PutUint32(head[8:12], uint32(len(metab)))
		binary.BigEndian.PutUint32(head[12:16], uint32(len(dat)))

		if _, err := w.Write(head); err != nil {
			return err
		}
		if _, err := w.Write(metab); err != nil {
			return err
		}
		if _, err := w.Write(dat); err != nil {
			return err
		}

		start += sizeDat

		if start >= len(p.Data) {
			break
		}
	}

	return nil
}
