package ws

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"github.com/libsdf/df/conf"
	"testing"
	"time"
)

func TestWsReadWrite(t *testing.T) {
	cfg := make(conf.Values)
	cfg.Set(conf.FRAMER_ROLE, "client")
	cfg.Set(conf.FRAMER_PSK, "XQhhqjIC5WOJtpKaXm40bQ==")
	cfg.Set(conf.FRAMER_TIMESTAMP, fmt.Sprintf("%d", time.Now().Unix()))

	dat := make([]byte, 75)
	rand.Read(dat)

	src := bytes.NewBuffer([]byte{})
	f0 := Framer(src, cfg)
	if _, err := f0.Write(dat); err != nil {
		t.Fatalf("%v", err)
		return
	}

	cfg1 := cfg.Clone()
	cfg1.Set(conf.FRAMER_ROLE, "server")
	f1 := Framer(bytes.NewBuffer(src.Bytes()), cfg1)
	buf := make([]byte, len(dat))
	if size, err := f1.Read(buf); err != nil {
		t.Fatalf("%v", err)
		return
	} else if size != len(dat) {
		t.Fatalf("unexpected read size: %d", size)
		return
	} else if !bytes.Equal(buf, dat) {
		t.Fatalf("unexpected data read: %v (expecting %v)", buf, dat)
	}
}

func TestWsReadWrite2(t *testing.T) {
	cfg := make(conf.Values)
	cfg.Set(conf.FRAMER_ROLE, "client")
	cfg.Set(conf.FRAMER_PSK, "XQhhqjIC5WOJtpKaXm40bQ==")
	cfg.Set(conf.FRAMER_TIMESTAMP, fmt.Sprintf("%d", time.Now().Unix()))

	dat := make([]byte, 374)
	rand.Read(dat)

	src := bytes.NewBuffer([]byte{})
	f0 := Framer(src, cfg)
	if _, err := f0.Write(dat); err != nil {
		t.Fatalf("%v", err)
		return
	}

	cfg1 := cfg.Clone()
	cfg1.Set(conf.FRAMER_ROLE, "server")
	f1 := Framer(bytes.NewBuffer(src.Bytes()), cfg1)
	buf := make([]byte, len(dat))
	if size, err := f1.Read(buf); err != nil {
		t.Fatalf("%v", err)
		return
	} else if size != len(dat) {
		t.Fatalf("unexpected read size: %d", size)
		return
	} else if !bytes.Equal(buf, dat) {
		t.Fatalf("unexpected data read: %v (expecting %v)", buf, dat)
	}
}

func TestWsReadWrite3(t *testing.T) {
	cfg := make(conf.Values)
	cfg.Set(conf.FRAMER_ROLE, "client")
	cfg.Set(conf.FRAMER_PSK, "XQhhqjIC5WOJtpKaXm40bQ==")
	cfg.Set(conf.FRAMER_TIMESTAMP, fmt.Sprintf("%d", time.Now().Unix()))

	dat := make([]byte, 1<<18+1)
	rand.Read(dat)

	src := bytes.NewBuffer([]byte{})
	f0 := Framer(src, cfg)
	if _, err := f0.Write(dat); err != nil {
		t.Fatalf("%v", err)
		return
	}

	cfg1 := cfg.Clone()
	cfg1.Set(conf.FRAMER_ROLE, "server")
	f1 := Framer(bytes.NewBuffer(src.Bytes()), cfg1)
	buf := make([]byte, len(dat))
	if size, err := f1.Read(buf); err != nil {
		t.Fatalf("%v", err)
		return
	} else if size != len(dat) {
		t.Fatalf("unexpected read size: %d", size)
		return
	} else if !bytes.Equal(buf, dat) {
		t.Fatalf("unexpected data read: %v (expecting %v)", buf, dat)
	}
}
