package http

import (
	"bytes"
	"fmt"
	"testing"
)

func TestReadResponse(t *testing.T) {
	rs := bytes.NewBuffer([]byte{})
	fmt.Fprintf(rs, "HTTP/1.1 200 OK\r\n")
	fmt.Fprintf(rs, "Connection: close\r\n")
	fmt.Fprintf(rs, "Content-Type: image/png,stream=1\r\n")
	fmt.Fprintf(rs, "Content-Transfer-Encoding: custom\r\n")
	fmt.Fprintf(rs, "\r\n")

	r := bytes.NewReader(rs.Bytes())
	resp, err := ReadResponse(r)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
		return
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status code: expects 200 but got %d", resp.StatusCode)
	}
	// if len(lines) != 4 {
	// 	t.Fatalf("expects 4 lines but got %d.", len(lines))
	// 	return
	// }
}
