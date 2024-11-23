/*
Package f1 contains 2 implementations of Framer, [F0] and [F1]. The later is the one with encryption. [F0] is just for test use.
*/
package f1

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/transport/framer"
	"io"
	"strconv"
	"sync"
)

type F1 struct {
	tx  io.ReadWriteCloser
	psk []byte

	bufRead    []byte
	lenBufRead int

	lckRead  *sync.Mutex
	lckWrite *sync.Mutex
}

func (f *F1) SetPSK(psk []byte) {
	f.psk = psk
}

func (f *F1) appendBuf(dat []byte) {
	sizeAfter := f.lenBufRead + len(dat)
	if sizeAfter > cap(f.bufRead) {
		buf := make([]byte, sizeAfter)
		copy(buf, f.bufRead[:f.lenBufRead])
		copy(buf[f.lenBufRead:], dat)
		f.bufRead = buf
		f.lenBufRead = sizeAfter
	} else {
		copy(f.bufRead[f.lenBufRead:], dat)
		f.lenBufRead = sizeAfter
	}
}

func (f *F1) Read(buf []byte) (int, error) {
	f.lckRead.Lock()
	defer f.lckRead.Unlock()

	// log.Debugf("Read(%d bytes buffer)", len(buf))
	if f.lenBufRead > 0 {
		size := len(buf)
		if size > f.lenBufRead {
			size = f.lenBufRead
		}
		copy(buf[:size], f.bufRead[:size])
		copy(f.bufRead, f.bufRead[size:])
		f.lenBufRead -= size
		return size, nil
	}

	header := make([]byte, 10)
	if _, err := io.ReadFull(f.tx, header); err != nil {
		return 0, err
	}

	sizeAll := int(binary.BigEndian.Uint32(header[2:6]))
	sizePad := int(binary.BigEndian.Uint32(header[6:10]))
	// log.Debugf("read chunk(size=%d, pad=%d)", sizeAll, sizePad)

	chunkEnc := make([]byte, sizeAll)
	if _, err := io.ReadFull(f.tx, chunkEnc); err != nil {
		return 0, err
	}

	chunk, err := Decrypt(f.psk, chunkEnc)
	if err != nil {
		return 0, err
	}

	chunk = chunk[:len(chunk)-sizePad]
	size := len(chunk)
	if size > len(buf) {
		size = len(buf)
		copy(buf, chunk[:size])
		f.appendBuf(chunk[size:])
		return size, nil
	}

	copy(buf[:size], chunk)
	return size, nil
}

func (f *F1) Write(dat []byte) (int, error) {
	f.lckWrite.Lock()
	defer f.lckWrite.Unlock()

	sizePad := 0
	if len(dat) < 256 {
		r := []byte{0}
		rand.Read(r)
		sizePad = int(r[0])
	}
	if sizePad < 2 {
		sizePad = 2
	}
	pad := make([]byte, sizePad)
	rand.Read(pad)

	chunk := make([]byte, len(dat)+sizePad)
	copy(chunk, dat)
	copy(chunk[len(dat):], pad)

	chunkEnc, err := Encrypt(f.psk, chunk)
	if err != nil {
		return 0, err
	}
	size := len(chunkEnc)

	header := make([]byte, 10)
	binary.BigEndian.PutUint32(header[2:6], uint32(size))
	binary.BigEndian.PutUint32(header[6:10], uint32(sizePad))
	sizeHeader := len(header)

	chunkFin := make([]byte, sizeHeader+size)
	copy(chunkFin, header)
	copy(chunkFin[sizeHeader:], chunkEnc)

	// log.Debugf("write chunk(size=%d, pad=%d)", size, sizePad)
	return f.tx.Write(chunkFin)
}

func (f *F1) Close() error {
	return f.tx.Close()
}

func Framer(tx io.ReadWriteCloser, cfg conf.Values) framer.Framer {
	psk := cfg.Get(conf.FRAMER_PSK)
	timestamp := cfg.Get(conf.FRAMER_TIMESTAMP)

	ts := int64(0)
	if iv, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		ts = iv
	}

	pskb, err := base64.StdEncoding.DecodeString(psk)
	if err != nil {
		panic(err)
	} else if len(pskb) <= 0 {
		panic(fmt.Errorf("empty PSK."))
	}

	keySize := KeySize()
	if len(pskb) > keySize {
		pskb = pskb[:keySize]
	} else if len(pskb) < keySize {
		k := make([]byte, keySize)
		for i := 0; i < keySize; i += len(pskb) {
			copy(k[i:], pskb)
		}
		pskb = k
	}

	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(ts))
	for i := 0; i < len(tsBytes) && i < len(pskb); i += 1 {
		pskb[i] = pskb[i] ^ tsBytes[i]
	}

	return &F1{
		tx:       tx,
		psk:      pskb,
		bufRead:  make([]byte, conf.DEFAULT_BUFFER_SIZE),
		lckRead:  new(sync.Mutex),
		lckWrite: new(sync.Mutex),
	}
}
