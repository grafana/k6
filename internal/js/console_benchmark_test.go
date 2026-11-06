package js

import (
	"io"
	"strings"
	"testing"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
)

// BenchmarkConsoleControlCharacters measures logging with and without characters that need escaping.
func BenchmarkConsoleControlCharacters(b *testing.B) {
	for _, tc := range []struct {
		name string
		text string
	}{
		{name: "Plain", text: "response body: hello world\n"},
		{name: "Control", text: "response body: \x1b[31mhello\rworld\x00\u0085\n"},
	} {
		for _, size := range []struct {
			name   string
			repeat int
		}{
			{name: "Short", repeat: 1},
			{name: "Long", repeat: 100},
		} {
			b.Run(tc.name+"/"+size.name, func(b *testing.B) {
				logger := logrus.New()
				logger.SetOutput(io.Discard)
				logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
				c := newConsole(logger)
				rt := sobek.New()
				text := strings.Repeat(tc.text, size.repeat)
				value := rt.ToValue(text)
				b.ReportAllocs()
				b.SetBytes(int64(len(text)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					c.Log(value)
				}
			})
		}
	}
}
