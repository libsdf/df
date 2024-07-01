package h1

import (
	"context"
	"fmt"
	"github.com/libsdf/df/conf"
	"io"
	"testing"
	"time"
)

func TestClientRW(t *testing.T) {
	x, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := 57183
	psk := "XQhhqjIC5WOJtpKaXm40bQ=="

	ch := make(chan []byte)

	paramsServer := make(conf.Values)
	paramsServer.Set(conf.FRAMER_PSK, psk)

	go Server(x, &ServerOptions{
		Port:           port,
		ProtocolParams: paramsServer,
		Handler: func(clientId string, conn io.ReadWriteCloser) {
			buf := make([]byte, 1024)
			for {
				if size, err := conn.Read(buf); err != nil {
					t.Fatalf("conn.Read: %v", err)
					return
				} else {
					fmt.Printf("server: %d bytes received.\r\n", size)
					chunk := make([]byte, size)
					copy(chunk, buf[:size])
					ch <- chunk
				}
			}
		},
	})

	serverUrl := fmt.Sprintf("http://localhost:%d/", port)
	params := make(conf.Values)
	params.Set(conf.SERVER_URL, serverUrl)
	params.Set(conf.FRAMER_PSK, psk)
	cl, err := CreateClient(params, "test1")
	if err != nil {
		t.Fatalf("CreateClient: %v", err)
		return
	}
	defer cl.Close()

	lines := []string{
		"hello",
		"world!",
	}
	for _, line := range lines {
		if size, err := cl.Write([]byte(line)); err != nil {
			t.Fatalf("cl.Write: %v", err)
			return
		} else if size != len(line) {
			t.Fatalf("wrong size returned from cl.Write: %d", size)
			return
		} else {
			fmt.Printf("client: %d bytes sent.\r\n", size)
		}
		select {
		case <-time.After(time.Second * 2):
			t.Fatalf("timeout receiving at server side.")
			return
		case lineReceived := <-ch:
			if string(lineReceived) != line {
				t.Fatalf("expects `%s` but got `%s`.", line, lineReceived)
			}
		}
	}

}
