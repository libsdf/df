package backend

import (
	"io"
	"sync"
)

type virtualConn struct {
	closed bool
	lck    *sync.RWMutex

	chR   chan []byte
	chW   chan []byte
	chErr chan error

	bufRead    []byte
	lenBufRead int
}

func (c *virtualConn) appendBuf(dat []byte) {
	sizeAfter := c.lenBufRead + len(dat)
	if sizeAfter > cap(c.bufRead) {
		buf := make([]byte, sizeAfter)
		copy(buf, c.bufRead[:c.lenBufRead])
		copy(buf[c.lenBufRead:], dat)
		c.bufRead = buf
		c.lenBufRead = sizeAfter
	} else {
		copy(c.bufRead[c.lenBufRead:], dat)
		c.lenBufRead = sizeAfter
	}
}

func (c *virtualConn) Read(buf []byte) (int, error) {
	if c.lenBufRead > 0 {
		size := len(buf)
		if size > c.lenBufRead {
			size = c.lenBufRead
		}
		copy(buf[:size], c.bufRead[:size])
		copy(c.bufRead, c.bufRead[size:])
		c.lenBufRead -= size
		return size, nil
	}

	if c.isClosed() {
		return 0, io.EOF
	}

	select {
	case chunk := <-c.chR:
		if chunk == nil {
			return 0, io.EOF
		}
		size := len(chunk)
		if len(chunk) > len(buf) {
			size = len(buf)
			copy(buf, chunk[:size])
			c.appendBuf(chunk[size:])
			return size, nil
		}
		copy(buf[:size], chunk)
		return size, nil
	}
}

func (c *virtualConn) Write(dat []byte) (int, error) {
	if c.isClosed() {
		return 0, io.EOF
	}
	if len(dat) > 0 {
		c.chW <- dat
	}
	return len(dat), nil
}

func (c *virtualConn) Close() error {
	c.chErr <- nil
	return nil
}

func (c *virtualConn) isClosed() bool {
	c.lck.RLock()
	defer c.lck.RUnlock()
	return c.closed
}

func (c *virtualConn) closeLocal() {
	c.lck.Lock()
	defer c.lck.Unlock()
	c.closed = true
}
