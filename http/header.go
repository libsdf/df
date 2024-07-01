package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Header struct {
	Method string
	Proto  string
	Host   string
	URL    *url.URL
	Values http.Header
}

func NewHeader() *Header {
	return &Header{
		Values: make(http.Header),
	}
}

func (h *Header) WriteTo(w io.Writer) (int, error) {
	total := 0
	size, err := fmt.Fprintf(
		w,
		"%s %s %s\r\n",
		strings.ToUpper(h.Method),
		h.URL.RequestURI(),
		h.Proto,
	)
	if err != nil {
		return total, err
	}
	total += size

	hostSet := false
	if len(h.Values) > 0 {
		for name, values := range h.Values {
			if strings.EqualFold(name, "host") {
				hostSet = true
			}
			for _, value := range values {
				size, err := fmt.Fprintf(
					w,
					"%s: %s\r\n",
					name,
					value,
				)
				if err != nil {
					return total, err
				}
				total += size
			}
		}
	}

	if !hostSet && len(h.URL.Host) > 0 {
		size, err := fmt.Fprintf(
			w,
			"Host: %s\r\n",
			h.URL.Host,
		)
		if err != nil {
			return total, err
		}
		total += size
	}

	size, err = fmt.Fprintf(w, "\r\n")
	if err != nil {
		return total, err
	}
	total += size
	return total, nil
}

func parseHeader(dat []byte) (*Header, error) {
	crlf := []byte("\r\n")
	sp := []byte(" ")
	lines := bytes.Split(dat, crlf)
	if len(lines) <= 0 {
		return nil, fmt.Errorf("Invalid http header.")
	}
	reqParts := bytes.SplitN(lines[0], sp, 3)
	if len(reqParts) != 3 {
		return nil, fmt.Errorf("Invalid http request.")
	}
	h := &Header{
		Values: make(http.Header),
	}
	h.Method = strings.ToUpper(string(reqParts[0]))
	h.Proto = strings.ToUpper(string(reqParts[2]))

	urlStr := string(reqParts[1])
	uri, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return nil, fmt.Errorf("Invalid request uri.")
	}

	for _, lineb := range lines[1:] {
		parts := bytes.SplitN(lineb, sp, 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimRight(string(parts[0]), ": ")
		value := string(parts[1])
		if strings.EqualFold(name, "host") {
			h.Host = value
		}
		h.Values.Set(name, value)
		// fmt.Printf("%s: %s\n", name, value)
	}

	if len(uri.Host) <= 0 && len(h.Host) > 0 {
		uri.Host = h.Host
	}
	h.URL = uri

	return h, nil
}
