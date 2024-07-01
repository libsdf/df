package backend

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

func RandDigits(size int) string {
	buff := make([]byte, size)
	rand.Read(buff)
	for i := 0; i < size; i += 1 {
		v := buff[i]
		buff[i] = byte(48 + int((float32(v)*10.0)/256.0))
	}
	return string(buff)
}

func EncodeAsASCII(buf []byte) string {
	for i := 0; i < len(buf); i += 1 {
		v := byte(float32(buf[i]) * 62.0 / 256.0)
		if v < 10 {
			v += 48
		} else if v < 10+26 {
			v += 97 - 10
		} else if v < 10+26+26 {
			v += 65 - 26 - 10
		}
		buf[i] = v
	}
	return string(buf)
}

func RandString(size int) string {
	buf := make([]byte, size)
	rand.Read(buf)
	return EncodeAsASCII(buf)
}

func UniqueId(size int) string {
	t := uint64(time.Now().Unix())
	tbuf := make([]byte, 8)
	binary.BigEndian.PutUint64(tbuf, t)
	buf := make([]byte, size)
	rand.Read(buf)
	for i := 0; i < len(tbuf) && i < len(buf); i += 1 {
		buf[i] = buf[i] ^ tbuf[i]
	}
	return EncodeAsASCII(buf)
	// for i := 0; i < len(buf); i += 1 {
	// 	v := byte(float32(buf[i]) * 62.0 / 256.0)
	// 	if v < 10 {
	// 		v += 48
	// 	} else if v < 10+26 {
	// 		v += 97 - 10
	// 	} else if v < 10+26+26 {
	// 		v += 65 - 26 - 10
	// 	}
	// 	buf[i] = v
	// }
	// return string(buf)
}
