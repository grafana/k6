package log

import (
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"
)

func parseLevels(level string) ([]logrus.Level, error) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("unknown log level %s", level) // specifically use a custom error
	}
	index := sort.Search(len(logrus.AllLevels), func(i int) bool {
		return logrus.AllLevels[i] > lvl
	})

	return logrus.AllLevels[:index], nil
}
