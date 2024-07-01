package log

import (
	"log"
	"os"
)

type defaultHandler struct {
	format string
	logger *log.Logger
}

func (h *defaultHandler) Format() string {
	return h.format
}

func (h *defaultHandler) SetFormat(format string) {
	h.format = format
}

func (h *defaultHandler) Write(msg *Message) {
	line := formatMessage(h.format, msg)
	h.logger.Print(line)
}

func (h *defaultHandler) CleanUp() {
}

func DefaultHandler() Handler {
	return &defaultHandler{
		format: "[$mod] <$filename:$lineno> $lev* $message",
		logger: log.New(os.Stdout, "", 0),
	}
}
