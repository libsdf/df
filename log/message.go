package log

import (
	"time"
)

type Message struct {
	Timestamp  time.Time `json:"timestamp"`
	LoggerName string    `json:"logger_name"`
	Message    string    `json:"message"`
	Lev        int       `json:"lev"`
	Mod        string    `json:"mod"`
	FileName   string    `json:"filename"`
	LineNo     int       `json:"line_no"`
}
