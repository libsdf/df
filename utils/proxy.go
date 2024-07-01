package utils

import (
	"context"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/log"
	"io"
	"sync"
)

func ProxyIOCopy(conn, upConn io.ReadWriter) {
	// chErr := make(chan error, 2)
	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		// down
		defer wg.Done()
		if _, err := io.Copy(conn, upConn); err != nil {
			// chErr <- nil
		}
	}()

	go func() {
		// up
		defer wg.Done()
		if _, err := io.Copy(upConn, conn); err != nil {
			// chErr <- nil
		}
	}()

	wg.Wait()
	// select {
	// case err := <- chErr:
	// 	if err != nil && !utils.IsIOError(err) {
	// 		log.Warnf("%v", err)
	// 	}
	// }
}

func Proxy(conn, upConn io.ReadWriter) {
	chErr := make(chan error, 4)
	chUp := make(chan []byte, 8)
	chDown := make(chan []byte, 8)

	x, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		// read local
		buf := make([]byte, conf.DEFAULT_BUFFER_SIZE)
		for {
			if size, err := conn.Read(buf); err != nil {
				chErr <- err
				return
			} else {
				chunk := make([]byte, size)
				copy(chunk, buf[:size])
				chUp <- chunk
			}
		}
	}()

	go func() {
		// read up-link
		buf := make([]byte, conf.DEFAULT_BUFFER_SIZE)
		for {
			if size, err := upConn.Read(buf); err != nil {
				chErr <- err
				return
			} else {
				chunk := make([]byte, size)
				copy(chunk, buf[:size])
				chDown <- chunk
			}
		}
	}()

	go func() {
		// write up-link
		for {
			select {
			case <-x.Done():
				return
			case chunk := <-chUp:
				if len(chunk) > 0 {
					if _, err := upConn.Write(chunk); err != nil {
						chErr <- err
						return
					}
				}
			}
		}
	}()

	for {
		select {
		case <-x.Done():
			return
		case err := <-chErr:
			if err != nil {
				if !IsIOError(err) {
					log.Warnf("%v", err)
				}
			}
			return
		case chunk := <-chDown:
			if len(chunk) > 0 {
				if _, err := conn.Write(chunk); err != nil {
					if !IsIOError(err) {
						log.Warnf("%v", err)
					}
					return
				}
			}
		}
	}
}
