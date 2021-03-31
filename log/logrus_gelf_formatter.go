package log

// Code based on the project: https://github.com/seatgeek/logrus-gelf-formatter

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/sirupsen/logrus"
)

// GelfFormatter defines a log format type that wil output line separated JSON objects in the GELF format.
type GelfFormatter struct{}

type fields map[string]interface{}

// Syslog severity levels
const (
	EmergencyLevel = int32(0)
	CriticalLevel  = int32(2)
	ErrorLevel     = int32(3)
	WarningLevel   = int32(4)
	NoticeLevel    = int32(5)
	InfoLevel      = int32(6)
	DebugLevel     = int32(7)
)

// Format formats the log entry to GELF JSON
func (f *GelfFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	data := make(fields, len(entry.Data)+6)
	blacklist := []string{"_id", "id", "timestamp", "version", "level"}

	for k, v := range entry.Data {
		if contains(k, blacklist) {
			continue
		}

		switch v := v.(type) {
		case error:
			// Otherwise errors are ignored by `encoding/json`
			data["_"+k] = v.Error()
		default:
			data["_"+k] = v
		}
	}

	data["version"] = "1.1"
	data["short_message"] = entry.Message
	data["timestamp"] = round((float64(entry.Time.UnixNano())/float64(1000000))/float64(1000), 4)
	data["level"] = toSyslogLevel(entry.Level)
	data["level_name"] = entry.Level.String()
	data["_pid"] = os.Getpid()

	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fields to JSON, %w", err)
	}

	return append(serialized, '\n'), nil
}

func contains(needle string, haystack []string) bool {
	for _, a := range haystack {
		if needle == a {
			return true
		}
	}
	return false
}

func round(val float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Floor((val*shift)+.5) / shift
}

func toSyslogLevel(l logrus.Level) int32 {
	var level int32
	switch l {
	case logrus.PanicLevel:
		level = EmergencyLevel
	case logrus.FatalLevel:
		level = CriticalLevel
	case logrus.ErrorLevel:
		level = ErrorLevel
	case logrus.WarnLevel:
		level = WarningLevel
	case logrus.InfoLevel:
		level = InfoLevel
	case logrus.DebugLevel:
		level = DebugLevel
	case logrus.TraceLevel:
		level = NoticeLevel
	}

	return level
}
