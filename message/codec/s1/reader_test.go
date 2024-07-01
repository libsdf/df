package s1

import (
	"bytes"
	"github.com/libsdf/df/message"
	"testing"
)

func TestReaderArrayHeader(t *testing.T) {
	r0 := bytes.NewReader([]byte{
		'a', 0, 0, 0, 1,
	})
	r := NewReader(r0)
	h, err := r.ReadArrayHeader()
	if err != nil {
		t.Fatalf("r.ReadArrayHeader: %v", err)
	} else {
		if h.Size != 1 {
			t.Fatalf("array size: expects 1 but got %d.", h.Size)
		}
	}
}

func TestReaderBytes(t *testing.T) {
	r0 := bytes.NewReader([]byte{
		'b', 0, 0, 0, 2, 'c', 'd',
	})
	r := NewReader(r0)
	dat, err := r.ReadBytes()
	if err != nil {
		t.Fatalf("r.ReadBytes: %v", err)
	} else {
		if len(dat) != 2 {
			t.Fatalf("bytes: expects 2 bytes but got %d.", len(dat))
		} else {
			if dat[0] != 'c' || dat[1] != 'd' {
				t.Fatalf("bytes: expects 'cd' but got %x", dat)
			}
		}
	}
}

func TestReaderString(t *testing.T) {
	r0 := bytes.NewReader([]byte{
		's', 0, 0, 0, 2, 'c', 'd',
	})
	r := NewReader(r0)
	dat, err := r.ReadString()
	if err != nil {
		t.Fatalf("r.ReadString: %v", err)
	} else {
		if len(dat) != 2 {
			t.Fatalf("str: expects 2 bytes but got %d.", len(dat))
		} else {
			if string(dat) != "cd" {
				t.Fatalf("str: expects \"cd\" but got %x", dat)
			}
		}
	}
}

func TestReadingResultDNS(t *testing.T) {
	dat := []byte{
		message.KindResultDNS, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, // id
		0, 0, 0, 1, // ack
		0, 0, 0, 19, // size
		'a', 0, 0, 0, 1,
		'b', 0, 0, 0, 4, 34, 117, 188, 166,
		's', 0, 0, 0, 0,
	}
	r0 := bytes.NewReader(dat)
	r := NewReader(r0)
	if m, err := r.Read(); err != nil {
		t.Fatalf("r.Read: %v", err)
	} else {
		m1 := m.(*message.MessageResultDNS)
		if len(m1.Addresses) != 1 {
			t.Fatalf("addresses: expects 1 but got %d.",
				len(m1.Addresses),
			)
		}
	}
}

func TestReadingMessageData(t *testing.T) {
	dat := []byte{
		message.KindData, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, // id
		0, 0, 0, 0, // ack
		0, 0, 0, 18, // size
		's', 0, 0, 0, 4, 't', 'e', 's', 't',
		'b', 0, 0, 0, 4, 'a', 'b', 'c', 'd',
	}
	r := NewReader(bytes.NewReader(dat))
	m, err := r.Read()
	if err != nil {
		t.Fatalf("r.Read: %v", err)
	} else {
		m1 := m.(*message.MessageData)
		if m1.ConnectionId != "test" {
			t.Fatalf("connection id: expects `%s` but got `%s`.",
				"test", m1.ConnectionId,
			)
			return
		}
		if string(m1.Data) != "abcd" {
			t.Fatalf("data: expects `%s` but got `%s`.",
				"abcd", string(m1.Data),
			)
		}
	}
}
