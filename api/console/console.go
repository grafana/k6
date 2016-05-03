package console

import (
	log "github.com/Sirupsen/logrus"
	"strconv"
)

var members = map[string]interface{}{
	"log":   Log,
	"warn":  Warn,
	"error": Error,
}

func New() map[string]interface{} {
	return members
}

func consoleLogFields(args []interface{}) log.Fields {
	fields := log.Fields{}
	for i, arg := range args {
		fields[strconv.Itoa(i+1)] = arg
	}
	return fields
}

// TODO: Match console.log()'s sprintf()-like formatting behavior
func Log(msg string, args ...interface{}) {
	log.WithFields(consoleLogFields(args)).Info(msg)
}

func Warn(msg string, args ...interface{}) {
	log.WithFields(consoleLogFields(args)).Warn(msg)
}

func Error(msg string, args ...interface{}) {
	log.WithFields(consoleLogFields(args)).Error(msg)
}
