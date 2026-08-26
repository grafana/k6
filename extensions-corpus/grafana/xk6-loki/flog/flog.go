package flog

import (
	"math/rand"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

type Flog struct {
	rand     *rand.Rand
	gofakeit *gofakeit.Faker
}

func New(rand *rand.Rand, faker *gofakeit.Faker) *Flog {
	return &Flog{rand: rand, gofakeit: faker}
}

func (f *Flog) LogLine(format string, t time.Time) string {
	switch format {
	case "apache_common":
		return f.NewApacheCommonLog(t)
	case "apache_combined":
		return f.NewApacheCombinedLog(t)
	case "apache_error":
		return f.NewApacheErrorLog(t)
	case "rfc3164":
		return f.NewRFC3164Log(t)
	case "rfc5424":
		return f.NewRFC5424Log(t)
	case "common_log":
		return f.NewCommonLogFormat(t)
	case "json":
		return f.NewJSONLogFormat(t)
	case "logfmt":
		return f.NewLogFmtLogFormat(t)
	default:
		return ""
	}
}
