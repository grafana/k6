package pb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO(imiric): Consider adding logging tests for 100% pb coverage.
// Unfortunately the following introduces an import cycle: pb -> lib -> pb
// func getTestLogger() *logger.Entry {
// 	logHook := testutils.NewLogHook(logrus.WarnLevel)
// 	testLog := logrus.New()
// 	testLog.AddHook(logHook)
// 	testLog.SetOutput(io.Discard)
// 	return logrus.NewEntry(testLog)
// }

func TestProgressBarRender(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		options      []ProgressBarOption
		pbWidthDelta int
		expected     string
	}{
		{
			[]ProgressBarOption{WithLeft(func() string { return "left" })},
			0, "left   [--------------------------------------]",
		},
		{
			[]ProgressBarOption{WithConstLeft("constLeft")},
			0, "constLeft   [--------------------------------------]",
		},
		{[]ProgressBarOption{
			WithLeft(func() string { return "left" }),
			WithStatus(Done),
		}, 0, "left ✓ [--------------------------------------]"},
		{[]ProgressBarOption{
			WithLeft(func() string { return "left" }),
			WithProgress(func() (float64, []string) { return 0, []string{"right"} }),
		}, 0, "left   [--------------------------------------] right"},
		{[]ProgressBarOption{
			WithLeft(func() string { return "left" }),
			WithProgress(func() (float64, []string) { return 0.5, []string{"right"} }),
		}, 0, "left   [==================>-------------------] right"},
		{[]ProgressBarOption{
			WithLeft(func() string { return "left" }),
			WithProgress(func() (float64, []string) { return 1.0, []string{"right"} }),
		}, 0, "left   [======================================] right"},
		{[]ProgressBarOption{
			WithLeft(func() string { return "left" }),
			WithProgress(func() (float64, []string) { return -1, []string{"right"} }),
		}, 0, "left   [--------------------------------------] right"},
		{[]ProgressBarOption{
			WithLeft(func() string { return "left" }),
			WithProgress(func() (float64, []string) { return 2, []string{"right"} }),
		}, 0, "left   [======================================] right"},
		{[]ProgressBarOption{
			WithLeft(func() string { return "left" }),
			WithConstProgress(0.2, "constProgress"),
		}, 0, "left   [======>-------------------------------] constProgress"},
		{[]ProgressBarOption{
			WithHijack(func() string { return "progressbar hijack!" }),
		}, 0, "progressbar hijack!"},
		{
			[]ProgressBarOption{WithConstProgress(0.25, "")},
			-DefaultWidth, "   [  25% ] ",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			pbar := New(tc.options...)
			assert.NotNil(t, pbar)
			assert.Equal(t, tc.expected, pbar.Render(0, tc.pbWidthDelta).String())
		})
	}
}

func TestProgressBarRenderPaddingMaxLeft(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		maxLen   int
		left     string
		expected string
	}{
		{-1, "left", "left   [--------------------------------------]"},
		{0, "left", "left   [--------------------------------------]"},
		{10, "left_truncated", "left_tr...   [--------------------------------------]"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.left, func(t *testing.T) {
			t.Parallel()
			pbar := New(WithLeft(func() string { return tc.left }))
			assert.NotNil(t, pbar)
			assert.Equal(t, tc.expected, pbar.Render(tc.maxLen, 0).String())
		})
	}
}

func TestProgressBarLeft(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		left     func() string
		expected string
	}{
		{nil, ""},
		{func() string { return " left " }, " left "},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			pbar := New(WithLeft(tc.left))
			assert.NotNil(t, pbar)
			assert.Equal(t, tc.expected, pbar.Left())
		})
	}
}
