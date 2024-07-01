/*
Package ws contains the implementation of Framer for websocket.
*/
package ws

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/transport/framer"
	"github.com/libsdf/df/transport/framer/f1"
	"io"
	"strconv"
	"strings"
)

func Encrypt(key, dat []byte) ([]byte, error) {
	// return dat, nil
	return f1.Encrypt(key, dat)
}

func Decrypt(key, dat []byte) ([]byte, error) {
	// return dat, nil
	return f1.Decrypt(key, dat)
}

type Ws struct {
	tx   io.ReadWriter
	psk  []byte
	mask bool // true for websocket client, false for server.

	bufRead    []byte
	lenBufRead int
}

func (f *Ws) SetPSK(psk []byte) {
	f.psk = psk
}

func (f *Ws) appendBuf(dat []byte) {
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

func (f *Ws) Read(buf []byte) (int, error) {
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

	header := make([]byte, 2)
	if _, err := io.ReadFull(f.tx, header); err != nil {
		return 0, err
	}

	fin := header[0]&(1<<7) > 0
	opCode := header[0] & 0b1111
	if !fin || opCode != 2 {
		// unlikely our frame.
		return 0, io.EOF
	}

	masking := header[1]&(1<<7) > 0
	sizeBase := header[1] & 0b1111111
	size := 0
	if sizeBase < 126 {
		size = int(sizeBase)
	} else if sizeBase == 126 {
		sizeb := make([]byte, 2)
		if _, err := io.ReadFull(f.tx, sizeb); err != nil {
			return 0, err
		}
		size = int(binary.BigEndian.Uint16(sizeb))
	} else {
		sizeb := make([]byte, 8)
		if _, err := io.ReadFull(f.tx, sizeb); err != nil {
			return 0, err
		}
		size = int(binary.BigEndian.Uint64(sizeb))
	}

	if masking == f.mask {
		return 0, fmt.Errorf("mask error in receiving frame.")
	}
	mask := ([]byte)(nil)
	if masking {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(f.tx, mask); err != nil {
			return 0, err
		}
	}

	chunkEnc := make([]byte, size)
	if _, err := io.ReadFull(f.tx, chunkEnc); err != nil {
		return 0, err
	}

	if len(mask) > 0 {
		for i, v := range chunkEnc {
			chunkEnc[i] = v ^ mask[i%4]
		}
	}

	chunk, err := Decrypt(f.psk, chunkEnc)
	if err != nil {
		return 0, err
	}

	if len(chunk) > len(buf) {
		f.appendBuf(chunk[len(buf):])
		copy(buf, chunk[:len(buf)])
		return len(buf), nil
	} else {
		copy(buf[:len(chunk)], chunk)
		return len(chunk), nil
	}
}

func (f *Ws) Write(dat []byte) (int, error) {
	chunkEnc, err := Encrypt(f.psk, dat)
	if err != nil {
		return 0, err
	}
	size := len(chunkEnc)

	sizeExtra := 0
	if size > 125 {
		if size >= 1<<16 {
			sizeExtra = 8
		} else {
			sizeExtra = 2
		}
	}

	maskSize := 0
	if f.mask {
		maskSize = 4
	}

	header := make([]byte, 2+sizeExtra+maskSize)
	header[0] = 0b10000010 /* [FIN _ _ _ OP(2)] */
	if size <= 125 {
		header[1] = byte(size)
	} else if sizeExtra == 2 {
		header[1] = byte(126)
		binary.BigEndian.PutUint16(header[2:4], uint16(size))
	} else if sizeExtra == 8 {
		header[1] = byte(127)
		binary.BigEndian.PutUint64(header[2:10], uint64(size))
	}
	if f.mask {
		header[1] = header[1] | (1 << 7)
	}

	if f.mask {
		mask := make([]byte, 4)
		rand.Read(mask)
		copy(header[len(header)-maskSize:], mask)
		for i, v := range chunkEnc {
			chunkEnc[i] = v ^ mask[i%4]
		}
	}

	chunkFin := make([]byte, len(header)+size)
	copy(chunkFin, header)
	copy(chunkFin[len(header):], chunkEnc)

	// log.Debugf("write chunk(size=%d, pad=%d)", size, sizePad)
	return f.tx.Write(chunkFin)
}

func (f *Ws) Close() error {
	if closer, ok := f.tx.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func Framer(tx io.ReadWriter, cfg conf.Values) framer.Framer {
	psk := cfg.Get(conf.FRAMER_PSK)
	timestamp := cfg.Get(conf.FRAMER_TIMESTAMP)
	role := cfg.Get(conf.FRAMER_ROLE)

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

	keySize := f1.KeySize()
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

	return &Ws{
		tx:      tx,
		psk:     pskb,
		mask:    strings.EqualFold(role, "client"),
		bufRead: make([]byte, conf.DEFAULT_BUFFER_SIZE),
	}
}
