package f1g

/* f1g: f1 with negotiation */

import (
	"crypto/rand"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/transport/framer/f1"
	"io"
	"strings"
	"sync"
)

type F1g struct {
	role string
	tx   io.ReadWriteCloser
	eof  bool
	lck  *sync.RWMutex
}

func (f *F1g) isEOF() bool {
	f.lck.RLock()
	defer f.lck.RUnlock()
	return f.eof
}

func (f *F1g) handshake() {
	f.lck.Lock()
	defer f.lck.Unlock()

	tx, ok := f.tx.(*f1.F1)
	if !ok {
		log.Warnf("tx is not *f1.F1.")
		f.eof = true
		return
	}

	lenKey := f1.KeySize()
	if strings.EqualFold(f.role, "server") {
		// offer session key to client.
		log.Debugf("sending session key...")
		key := f1.NewKey()
		chunk := make([]byte, lenKey*2)
		rand.Read(chunk)
		// nonce := chunk[:lenKey]
		copy(chunk[lenKey:], key)
		if size, err := tx.Write(chunk); err != nil {
			log.Warnf("tx.Write(%d bytes): %v", len(chunk), err)
			f.eof = true
			return
		} else {
			if size < len(chunk) {
				log.Warnf("chunk is %d bytes but only %d wrote.",
					len(chunk), size,
				)
			}
		}

		log.Debugf("new key: %x", key)
		tx.SetPSK(key)

		// wait ack from client.
		// log.Debugf("waiting for ack...")
		// ack := make([]byte, len(nonce))
		// if _, err := io.ReadFull(tx, ack); err != nil {
		// 	log.Warnf("tx.Read(ack: %d bytes): %v", len(ack), err)
		// 	f.eof = true
		// 	return
		// }
		// if !bytes.Equal(nonce, ack) {
		// 	log.Warnf("nonce != ack")
		// 	f.eof = true
		// 	return
		// }
		log.Debugf("handshake ok.")
	} else {
		// get session key from server.
		log.Debugf("waiting for session key from server...")
		chunk := make([]byte, lenKey*2)
		if _, err := io.ReadFull(tx, chunk); err != nil {
			log.Warnf("tx.Read(%d bytes): %v", len(chunk), err)
			f.eof = true
			return
		}
		// nonce := chunk[:lenKey]
		key := chunk[lenKey:]
		log.Debugf("new key: %x", key)
		tx.SetPSK(key)

		// // send ack
		// log.Debugf("sending ack to server...")
		// if size, err := tx.Write(nonce); err != nil {
		// 	log.Warnf("tx.Write(%d bytes): %v", len(nonce), err)
		// 	f.eof = true
		// 	return
		// } else {
		// 	if size < len(nonce) {
		// 		log.Warnf("data is %d bytes but only %d wrote.",
		// 			len(nonce), size,
		// 		)
		// 	}
		// }
		log.Debugf("handshake done.")
	}
}

func (f *F1g) Read(buf []byte) (int, error) {
	if f.isEOF() {
		return 0, io.EOF
	}
	return f.tx.Read(buf)
}

func (f *F1g) Write(dat []byte) (int, error) {
	if f.isEOF() {
		return 0, io.EOF
	}
	return f.tx.Write(dat)
}

func (f *F1g) Close() error {
	return f.tx.Close()
}

func Framer(tx io.ReadWriteCloser, cfg conf.Values) io.ReadWriteCloser {

	role := cfg.Get(conf.FRAMER_ROLE)
	txF1 := f1.Framer(tx, cfg)

	txF1g := &F1g{
		role: role,
		tx:   txF1,
		lck:  new(sync.RWMutex),
	}
	txF1g.handshake()

	return txF1g
}
