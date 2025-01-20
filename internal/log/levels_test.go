package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Test_getLevels(t *testing.T) {
	t.Parallel()

	tests := [...]struct {
		level  string
		err    bool
		levels []logrus.Level
	}{
		{
			level: "info",
			err:   false,
			levels: []logrus.Level{
				logrus.PanicLevel,
				logrus.FatalLevel,
				logrus.ErrorLevel,
				logrus.WarnLevel,
				logrus.InfoLevel,
			},
		},
		{
			level: "error",
			err:   false,
			levels: []logrus.Level{
				logrus.PanicLevel,
				logrus.FatalLevel,
				logrus.ErrorLevel,
			},
		},
		{
			level:  "tea",
			err:    true,
			levels: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.level, func(t *testing.T) {
			t.Parallel()

			levels, err := parseLevels(test.level)

			if test.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.levels, levels)
		})
	}
}
