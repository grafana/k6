package errext

import (
	"errors"

	"github.com/sirupsen/logrus"
)

func Fprint(l logrus.FieldLogger, err error) {
	if err == nil {
		return
	}

	errText := err.Error()
	var xerr Exception
	if errors.As(err, &xerr) {
		errText = xerr.StackTrace()
	}

	fields := logrus.Fields{}
	var herr HasHint
	if errors.As(err, &herr) {
		fields["hint"] = herr.Hint()
	}

	l.WithFields(fields).Error(errText)
}
