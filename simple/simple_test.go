package simple

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"testing"
)

func TestNew(t *testing.T) {
	r := New("http://example.com/")
	assert.Equal(t, "http://example.com/", r.URL)
}

func TestNewVU(t *testing.T) {
	r := New("http://example.com/")
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.IsType(t, &VU{}, vu)
}

func TestReconfigure(t *testing.T) {
	r := New("http://example.com/")

	vu, err := r.NewVU()
	assert.NoError(t, err)

	err = vu.Reconfigure(12345)
	assert.NoError(t, err)
}

func TestRunOnceReportsStats(t *testing.T) {
	r := New("http://255.255.255.255/")
	vu, err := r.NewVU()
	assert.NoError(t, err)

	err = vu.RunOnce(context.Background())
	assert.Error(t, err)

	mRequestsFound := false
	mErrrosFound := false
	for _, p := range vu.(*VU).Collector.Batch {
		switch p.Stat {
		case &mRequests:
			mRequestsFound = true
			assert.Contains(t, p.Tags, "url")
			assert.Contains(t, p.Tags, "method")
			assert.Contains(t, p.Tags, "status")
			assert.Contains(t, p.Values, "duration")
		case &mErrors:
			mErrrosFound = true
			assert.Contains(t, p.Tags, "url")
			assert.Contains(t, p.Tags, "method")
			assert.Contains(t, p.Tags, "status")
			assert.Contains(t, p.Values, "value")
		}
	}
	assert.True(t, mRequestsFound)
	assert.True(t, mErrrosFound)
}
