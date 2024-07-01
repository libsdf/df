package log

import (
	"fmt"
	"io"
	"time"
)

func SeverityFromLevel(lev int) int {
	switch lev {
	case CRITICAL:
		return 2
	case ERROR:
		return 3
	case WARNING:
		return 4
	case INFO:
		return 6
	case DEBUG:
		fallthrough
	default:
		return 7
	}
}

type SyslogEntry interface {
	io.WriterTo
}

type EntryRFC3164 struct {
	// Facility
	//   0: kenel
	//   1: user-level
	//   2~15: ...
	//   16~23: local use
	//   24+: ...
	Facility int

	// Severity
	//   0: emergency
	//   1: alert
	//   2: critical
	//   3: error
	//   4: warning
	//   5: notice
	//   6: informational
	//   7: debug
	Severity int

	// Timestamp will formated to as "Jan  2 15:04:05"
	Timestamp time.Time

	// Hostname is hostname or ip. It must not contains spaces.
	Hostname string

	// Tag has maximum size limit of 32.
	Tag     string
	Pid     int
	Content string
}

func (m *EntryRFC3164) WriteTo(w io.Writer) (int64, error) {
	pri := m.Facility*8 + m.Severity
	ts := m.Timestamp.Format(time.Stamp)
	var prefix string
	if m.Pid > 0 {
		prefix = fmt.Sprintf("%s[%d]:", m.Tag, m.Pid)
	} else {
		prefix = fmt.Sprintf("%s:", m.Tag)
	}
	size, err := fmt.Fprintf(
		w,
		"<%d>%s %s %s %s",
		pri, ts, m.Hostname, prefix, m.Content,
	)
	return int64(size), err
}

type EntryRFC5424 struct {
	// Facility
	//   0: kenel
	//   1: user-level
	//   2~15: ...
	//   16~23: local use
	//   24+: ...
	Facility int

	// Severity
	//   0: emergency
	//   1: alert
	//   2: critical
	//   3: error
	//   4: warning
	//   5: notice
	//   6: informational
	//   7: debug
	Severity int

	// Timestamp will formated to as "Jan  2 15:04:05"
	Timestamp time.Time

	// Hostname is hostname or ip. It must not contains spaces.
	Hostname string

	// Tag has maximum size limit of 32.
	App     string
	Pid     int
	Content string
}

func (m *EntryRFC5424) WriteTo(w io.Writer) (int64, error) {
	pri := m.Facility*8 + m.Severity
	ver := 1 // const 1 for RFC5424
	ts := m.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
	var app string
	if len(m.App) > 0 {
		app = m.App
	} else {
		app = "-"
	}
	msgId := "-"
	sd := "-"
	size, err := fmt.Fprintf(
		w,
		"<%d>%d %s %s %s %d %s %s %s",
		pri, ver, ts, m.Hostname, app, m.Pid, msgId, sd, m.Content,
	)
	return int64(size), err
}
