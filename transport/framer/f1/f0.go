package f1

import (
	"io"
)

type F0 struct {
	tx io.ReadWriteCloser
}

func (f *F0) Read(buf []byte) (int, error) {
	return f.tx.Read(buf)
}

func (f *F0) Write(buf []byte) (int, error) {
	return f.tx.Write(buf)
}

func (f *F0) Close() error {
	return f.tx.Close()
}
