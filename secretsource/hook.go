package secretsource

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// secretsHook is a Logrus hook for hiding secrets from entries before they get logged
type secretsHook struct {
	secrets  []string
	mx       sync.RWMutex
	replacer *strings.Replacer
}

// Levels is part of the [logrus.Hook]
func (s *secretsHook) Levels() []logrus.Level { return logrus.AllLevels }

// Add is used to add a new secret to be redacted.
// Adding the same secret multiple times will not error, but is not recommended.
// It is users job to not keep adding the same secret over time but only once.
func (s *secretsHook) add(secret string) {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.secrets = append(s.secrets, secret, "***SECRET_REDACTED***")
	s.replacer = strings.NewReplacer(s.secrets...)
}

// Fire is part of the [logrus.Hook]
func (s *secretsHook) Fire(entry *logrus.Entry) error {
	s.mx.Lock()
	// there is no way for us to get a secret after we got a log for it so we can use that to cache the replacer
	replacer := s.replacer
	s.mx.Unlock()
	if replacer == nil { // no secrets no work
		return nil
	}
	entry.Message = replacer.Replace(entry.Message)

	// replace both keys and values with
	for k, v := range entry.Data {
		newk := replacer.Replace(k)
		if newk != k {
			entry.Data[newk] = v
			delete(entry.Data, k)
			k = newk
		}
		entry.Data[k] = recursiveReplace(v, replacer)
	}

	return nil
}

func recursiveReplace(v any, replacer *strings.Replacer) any {
	switch s := v.(type) {
	case string:
		return replacer.Replace(s)
	case int, uint, int64, int32, int16, int8, uint64, uint32, uint16, uint8, float32, float64:
		// if the secret is encodable in 64 bits ... it is probably not a great secret
		return v
	case time.Duration:
		return v
	}
	return fmt.Sprintf("Had a logrus.fields value with type %T, "+
		"please report that this is unsupported and will be redacted in all logs in case it contains secrets", v)
}
