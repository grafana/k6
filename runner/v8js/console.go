package v8js

import (
	log "github.com/Sirupsen/logrus"
	"strconv"
)

func consoleLogFields(args []interface{}) log.Fields {
	fields := log.Fields{}
	for i, arg := range args {
		fields[strconv.Itoa(i+1)] = arg
	}
	return fields
}

// TODO: Match console.log()'s sprintf()-like formatting behavior
func (vu *VUContext) ConsoleLog(msg string, args ...interface{}) {
	log.WithFields(consoleLogFields(args)).Info(msg)
}

func (vu *VUContext) ConsoleWarn(msg string, args ...interface{}) {
	log.WithFields(consoleLogFields(args)).Warn(msg)
}

func (vu *VUContext) ConsoleError(msg string, args ...interface{}) {
	log.WithFields(consoleLogFields(args)).Error(msg)
}
