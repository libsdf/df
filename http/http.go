// Package http implements methods parsing http request and response.
package http

import (
	"bytes"
	"fmt"
	"io"
)

var (
	DefaultBufferSize = 1024 * 4
	ExtendingSize     = 1024 * 2
	MaxBufferSize     = 1024 * 16
)

type HeaderReader struct {
	r    io.Reader
	buff []byte
	end  int
}

func NewHeaderReader(r io.Reader) *HeaderReader {
	return &HeaderReader{
		r:    r,
		buff: make([]byte, DefaultBufferSize),
		end:  0,
	}
}

func (r *HeaderReader) readMore() (int, error) {
	readingSize := ExtendingSize
	for len(r.buff)-r.end < readingSize {
		if len(r.buff)+ExtendingSize > MaxBufferSize {
			return 0, io.ErrShortBuffer
		}
		buff := make([]byte, len(r.buff)+ExtendingSize)
		copy(buff[:r.end], r.buff[:r.end])
		r.buff = buff
	}
	if size, err := r.r.Read(r.buff[r.end:]); err != nil {
		return 0, err
	} else {
		r.end += size
		return size, nil
	}
}

func (r *HeaderReader) PeekHeader() (*Header, error) {
	sep := []byte("\r\n\r\n")
	for {
		p := bytes.Index(r.buff, sep)
		if p < 0 {
			if size, err := r.readMore(); err != nil {
				return nil, err
			} else if size <= 0 {
				return nil, io.EOF
			}
			continue
		}
		headerb := make([]byte, p)
		copy(headerb, r.buff[:p])
		if h, err := parseHeader(headerb); err != nil {
			return nil, fmt.Errorf("parseHeader: %v", err)
		} else {
			return h, nil
		}
	}
}

func (r *HeaderReader) Read(buff []byte) (int, error) {
	if r.end <= 0 {
		return r.r.Read(buff)
	}
	size := r.end
	if size > len(buff) {
		size = len(buff)
	}
	copy(buff[:size], r.buff[:size])
	newEnd := r.end - size
	copy(r.buff[:newEnd], r.buff[size:r.end])
	r.end = newEnd
	return size, nil
}
