package utils

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

const BUF_SIZE = 1024 * 4
const BUF_SIZE_MAX = 1024 * 1024 * 8

type ReadWriteBuffer struct {
	lck    *sync.Mutex
	buf    []byte
	bufLen int
	cond   *sync.Cond
	eof    *atomic.Bool
}

var _ io.ReadWriteCloser = (*ReadWriteBuffer)(nil)

func NewReadWriteBuffer() *ReadWriteBuffer {
	return &ReadWriteBuffer{
		lck:    new(sync.Mutex),
		buf:    make([]byte, BUF_SIZE),
		bufLen: 0,
		cond: &sync.Cond{
			L: new(sync.Mutex),
		},
		eof: new(atomic.Bool),
	}
}

func (f *ReadWriteBuffer) Close() error {
	if closed := f.eof.Swap(true); closed {
		return fmt.Errorf("buffer was closed already.")
	}
	return nil
}

func (f *ReadWriteBuffer) readFromBuf(buf []byte) int {
	f.lck.Lock()
	defer f.lck.Unlock()

	size := len(buf)
	if f.bufLen > 0 {
		if size > f.bufLen {
			size = f.bufLen
		}
		sizeRest := f.bufLen - size
		copy(buf[:size], f.buf[:size])
		copy(f.buf[0:sizeRest], f.buf[size:f.bufLen])
		f.bufLen = sizeRest
		return size
	}

	return 0
}

func (f *ReadWriteBuffer) Read(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	if size := f.readFromBuf(buf); size > 0 {
		return size, nil
	}

	if f.eof.Load() {
		return 0, io.EOF
	}

	for {
		f.cond.L.Lock()
		defer f.cond.L.Unlock()
		f.cond.Wait()
		if size := f.readFromBuf(buf); size > 0 {
			return size, nil
		}
	}
}

func (f *ReadWriteBuffer) Write(dat []byte) (int, error) {
	if f.eof.Load() {
		return 0, io.EOF
	}

	f.lck.Lock()
	defer f.lck.Unlock()

	sizeAfter := f.bufLen + len(dat)
	if sizeAfter > cap(f.buf) {
		// extend the buffer
		capNew := cap(f.buf) + BUF_SIZE
		for {
			if capNew < sizeAfter {
				capNew += BUF_SIZE
				if capNew > BUF_SIZE_MAX {
					return 0, io.ErrShortBuffer
				}
			} else {
				break
			}
		}
		bufnew := make([]byte, capNew)
		copy(bufnew[:f.bufLen], f.buf[:f.bufLen])
		f.buf = bufnew
	}
	copy(f.buf[f.bufLen:sizeAfter], dat)
	f.bufLen = sizeAfter

	f.cond.Broadcast()

	return len(dat), nil
}
