package log

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var ptVar = regexp.MustCompile(`(?is)\$(\w+[*]?)`)
var dateFmt = "2006-01-02 15:04:05"

func formatMessage(format string, msg *Message) string {
	if len(format) <= 0 || strings.EqualFold(format, "@json") {
		if dat, err := json.Marshal(msg); err == nil {
			return string(dat)
		}
		return ""
	}
	return ptVar.ReplaceAllStringFunc(format, func(match string) string {
		key := match[1:]
		switch key {
		case "time":
			return msg.Timestamp.Format(dateFmt)
		case "name":
			return msg.LoggerName
		case "message":
			return msg.Message
		case "lev":
			return GetLevelName(msg.Lev)
		case "lev*":
			return GetLevelNameColored(msg.Lev)
		case "mod":
			return msg.Mod
		case "filename":
			return msg.FileName
		case "lineno":
			return fmt.Sprintf("%d", msg.LineNo)
		}
		return match
	})
}
