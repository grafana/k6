package js

import (
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/robertkrimen/otto"
)

type Console struct {
	Logger *log.Logger
}

func (c Console) Log(level int, msg string, args []otto.Value) {
	fields := make(log.Fields, len(args))
	for i, arg := range args {
		if arg.IsObject() {
			obj := arg.Object()
			for _, key := range obj.Keys() {
				v, _ := obj.Get(key)
				fields[key] = v.String()
			}
			continue
		}
		fields["arg"+strconv.Itoa(i)] = arg.String()
	}

	entry := c.Logger.WithFields(fields)
	switch level {
	case 0:
		entry.Debug(msg)
	case 1:
		entry.Info(msg)
	case 2:
		entry.Warn(msg)
	case 3:
		entry.Error(msg)
	}
}
