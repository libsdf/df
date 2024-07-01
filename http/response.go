package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type Response struct {
	StatusCode int
	Header     http.Header
}

func parseResponseLine(line []byte) (int, string, string, error) {
	sp := []byte{' '}
	parts := bytes.SplitN(line, sp, 3)
	if len(parts) != 3 {
		return 0, "", "", fmt.Errorf("invalid response line.")
	}
	proto := string(parts[0])
	codeStr := string(parts[1])
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		return 0, "", "", err
	}
	statusText := string(parts[2])
	return code, statusText, proto, nil
}

func parseHeaderLine(line []byte) (string, string, error) {
	colon := []byte{':'}

	p := bytes.Index(line, colon)
	if p < 0 {
		return "", "", fmt.Errorf("invalid header line.")
	}
	name := strings.TrimSpace(string(line[:p]))
	value := strings.TrimSpace(string(line[p+1:]))
	return name, value, nil
}

func ReadResponse(conn io.Reader) (*Response, error) {
	lineIndex := 0
	bufHeader := make([]byte, 1024*16)
	i := 0
	buf := make([]byte, 1024)
	crlf := []byte{'\r', '\n'}
	rs := &Response{Header: make(http.Header)}
	for {
		if size, err := conn.Read(buf); err != nil {
			return nil, err
		} else {
			if i+size > cap(bufHeader) {
				return nil, io.ErrShortBuffer
			}
			copy(bufHeader[i:], buf[:size])
			i += size
			// check crlf
			for {
				p := bytes.Index(bufHeader[:i], crlf)
				if p < 0 {
					break
				}
				line := make([]byte, p)
				copy(line, bufHeader[:p])
				p += len(crlf)
				copy(bufHeader[:i-p], bufHeader[p:i])
				i -= p
				if len(line) > 0 {
					if lineIndex == 0 {
						statusCode, _, _, err := parseResponseLine(line)
						if err != nil {
							return nil, err
						}
						rs.StatusCode = statusCode
					} else {
						k, v, err := parseHeaderLine(line)
						if err != nil {
							return nil, err
						}
						rs.Header.Set(k, v)
					}
					lineIndex += 1
					// lines = append(lines, line)
				} else {
					//
					// break
					return rs, nil
				}
			}
		}
	}

	// line0 := lines[0]
	// lines = lines[1:]
	// statusCode, _, _, err := parseResponseLine(line0)
	// if err != nil {
	// 	return nil, err
	// }

	// rs.StatusCode = statusCode
	// for _, line := range lines {
	// 	if name, value, err := parseHeaderLine(line); err == nil {
	// 		rs.Header.Set(name, value)
	// 	}
	// }

	// return lines, nil
	return rs, nil
}
