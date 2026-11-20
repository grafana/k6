package common

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestNewJSHandleNilRemoteObject(t *testing.T) {
	t.Parallel()

	execCtx, ctx := newExecCtx()
	handle := NewJSHandle(ctx, nil, execCtx, nil, nil, log.NewNullLogger())

	require.NotNil(t, handle)
	require.Nil(t, handle.AsElement())
	require.Empty(t, handle.ObjectID())
}

// getPropertiesSession returns canned Runtime.getProperties results.
type getPropertiesSession struct {
	*Session
	descriptors []*runtime.PropertyDescriptor
}

func (s *getPropertiesSession) Execute(
	_ context.Context, _ string, _, res any,
) error {
	if ret, ok := res.(*runtime.GetPropertiesReturns); ok {
		ret.Result = s.descriptors
	}
	return nil
}

func TestGetPropertiesSkipsNilValue(t *testing.T) {
	t.Parallel()

	execCtx, ctx := newExecCtx()
	logger := log.NewNullLogger()
	sess := &getPropertiesSession{
		Session: &Session{id: "s", done: make(chan struct{}), logger: logger},
		descriptors: []*runtime.PropertyDescriptor{
			{
				Name:       "foo",
				Enumerable: true,
				Value:      nil, // accessor / getter: CDP omits value
				Get:        &runtime.RemoteObject{Type: runtime.TypeFunction},
			},
			{
				Name:       "bar",
				Enumerable: true,
				Value:      &runtime.RemoteObject{Type: runtime.TypeString},
			},
			{
				Name:       "hidden",
				Enumerable: false,
				Value:      &runtime.RemoteObject{Type: runtime.TypeString},
			},
		},
	}

	handle := NewJSHandle(
		ctx, sess, execCtx, nil,
		&runtime.RemoteObject{Type: runtime.TypeObject, ObjectID: "obj"},
		logger,
	)

	props, err := handle.GetProperties()
	require.NoError(t, err)
	require.Contains(t, props, "bar")
	require.NotContains(t, props, "foo", "accessor properties without a value must not crash or appear")
	require.NotContains(t, props, "hidden")
}
