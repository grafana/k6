package k6pack

import (
	"strings"
	"text/scanner"

	"github.com/evanw/esbuild/pkg/api"
)

type packError struct {
	messages []api.Message
}

func checkError(result *api.BuildResult) (bool, error) {
	if len(result.Errors) == 0 {
		return false, nil
	}

	return true, &packError{messages: result.Errors}
}

func wrapError(err error) error {
	return &packError{[]api.Message{{Text: err.Error()}}}
}

func (e *packError) Error() string {
	msg := e.messages[0]

	if msg.Location == nil {
		return msg.Text
	}

	pos := scanner.Position{
		Filename: msg.Location.File,
		Line:     msg.Location.Line,
		Column:   msg.Location.Column,
	}

	return pos.String() + " " + msg.Text
}

func (e *packError) Format(width int, color bool) string {
	return strings.Join(
		api.FormatMessages(
			e.messages,
			api.FormatMessagesOptions{TerminalWidth: width, Color: color},
		),
		"\n",
	)
}
