package log

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

type SyslogOptions struct {
	Host   string `yaml:"host" json:"host"`
	Port   int    `yaml:"port" json:"port"`
	Tag    string `yaml:"tag" json:"tag"`
	Format string `yaml:"format" json:"format"`
}

type canceler func()

type syslogHandler struct {
	options *SyslogOptions
	ch      chan *Message
	cancel  context.CancelFunc
	format  string
}

func NewSyslogHandler(options *SyslogOptions) Handler {
	format := options.Format
	x, cancel := context.WithCancel(context.Background())
	h := &syslogHandler{
		options: options,
		cancel:  cancel,
		format:  format,
	}
	go h.worker(x)
	return h
}

func (h *syslogHandler) worker(x context.Context) {
	addr := fmt.Sprintf("%s:%d", h.options.Host, h.options.Port)
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Printf("net.ResolveUDPAddr(%s): %v\n", addr, err)
		return
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		log.Printf("net.DialUDP(%s): %v\n", addr, err)
		return
	}
	defer conn.Close()

	h.ch = make(chan *Message, 1024)
	hostname, err := os.Hostname()
	if err != nil {
		println(err.Error())
		hostname = "unknown"
	}

	for {
		select {
		case <-x.Done():
			return
		case msg := <-h.ch:
			if msg != nil {
				msgLine := formatMessage(h.format, msg)
				syslogMsg := &EntryRFC5424{
					Facility:  1,
					Severity:  SeverityFromLevel(msg.Lev),
					Timestamp: time.Now(),
					Hostname:  hostname,
					App:       h.options.Tag,
					Pid:       os.Getpid(),
					Content:   msgLine,
				}
				// syslogMsg := &EntryRFC3164{
				// 	Facility: 1,
				// 	Severity: SeverityFromLevel(msg.Lev),
				// 	Timestamp: time.Now(),
				// 	Hostname: hostname,
				// 	Tag: h.options.Tag,
				// 	Pid: os.Getpid(),
				// 	Content: msgLine,
				// }
				if _, err := syslogMsg.WriteTo(conn); err != nil {
					println(err.Error())
				}
			}
		}
	}
}

func (h *syslogHandler) Format() string {
	return h.format
}

func (h *syslogHandler) SetFormat(format string) {
	h.format = format
}

func (h *syslogHandler) Write(msg *Message) {
	if msg != nil && h.ch != nil {
		h.ch <- msg
	}
}

func (h *syslogHandler) CleanUp() {
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
}
