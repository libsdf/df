package s1

import (
	"bytes"
	"github.com/libsdf/df/message"
	"testing"
)

func TestReadWrite1(t *testing.T) {
	req := &message.MessageRequestDNS{
		Id:         1,
		Network:    "ip",
		DomainName: "chat.0xff6600.com",
	}
	buf := bytes.NewBuffer([]byte{})
	w := NewWriter(buf)
	if _, err := w.Write(req); err != nil {
		t.Fatal(err)
	} else {
		dat := buf.Bytes()
		r0 := bytes.NewReader(dat)
		r := NewReader(r0)
		if m, err := r.Read(); err != nil {
			t.Fatalf("r.Read: %v", err)
		} else {
			m1 := m.(*message.MessageRequestDNS)
			if m1.Network != req.Network {
				t.Fatalf("network: expects `%s` but got `%s`.",
					req.Network, m1.Network,
				)
			} else if m1.DomainName != req.DomainName {
				t.Fatalf("domainName: expects `%s` but got `%s`.",
					req.DomainName, m1.DomainName,
				)
			}
		}
	}
}

func TestReadWrite2(t *testing.T) {
	rs := &message.MessageResultDNS{
		Ack:   1,
		Error: "",
		Addresses: [][]byte{
			{34, 117, 188, 166},
		},
	}
	buf := bytes.NewBuffer([]byte{})
	w := NewWriter(buf)
	if _, err := w.Write(rs); err != nil {
		t.Fatal(err)
	} else {
		dat := buf.Bytes()
		datEx := []byte{
			message.KindResultDNS, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, // id
			0, 0, 0, 1, // ack
			0, 0, 0, 19, // size
			'a', 0, 0, 0, 1,
			'b', 0, 0, 0, 4, 34, 117, 188, 166,
			's', 0, 0, 0, 0,
		}
		if !bytes.Equal(dat, datEx) {
			t.Fatalf("writer result wrong: %x", dat)
			return
		}
	}
}

func TestWriteMessageData(t *testing.T) {
	rs := &message.MessageData{
		ConnectionId: "test",
		Data:         []byte{'a', 'b', 'c', 'd'},
	}
	buf := bytes.NewBuffer([]byte{})
	w := NewWriter(buf)
	if _, err := w.Write(rs); err != nil {
		t.Fatal(err)
	} else {
		dat := buf.Bytes()
		datEx := []byte{
			message.KindData, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, // id
			0, 0, 0, 0, // ack
			0, 0, 0, 18, // size
			's', 0, 0, 0, 4, 't', 'e', 's', 't',
			'b', 0, 0, 0, 4, 'a', 'b', 'c', 'd',
		}
		if !bytes.Equal(dat, datEx) {
			t.Fatalf("writer result wrong: %x", dat)
			return
		}
	}
}
