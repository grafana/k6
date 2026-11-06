package common

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// handleReleaseSession supplies selector results and records releases at the CDP boundary.
// Unexpected commands fail rather than silently making the simulated browser succeed.
type handleReleaseSession struct {
	session
	lookupErr  error
	releaseErr error
	missing    bool
	released   []runtime.RemoteObjectID
	events     []string
}

func (s *handleReleaseSession) Execute(_ context.Context, method string, params, result any) error {
	switch method {
	case runtime.CommandCallFunctionOn:
		p := params.(*runtime.CallFunctionOnParams)
		switch {
		case strings.Contains(p.FunctionDeclaration, "injected.waitForSelector"):
			if s.lookupErr != nil {
				return s.lookupErr
			}
			if !s.missing {
				result.(*runtime.CallFunctionOnReturns).Result = &runtime.RemoteObject{
					Type: runtime.TypeObject, Subtype: runtime.SubtypeNode, ObjectID: "temporary-element",
				}
			}
		case strings.Contains(p.FunctionDeclaration, "injected.waitForElementStates"):
			result.(*runtime.CallFunctionOnReturns).Result = &runtime.RemoteObject{
				Type: runtime.TypeBoolean, Value: []byte("true"),
			}
		case strings.Contains(p.FunctionDeclaration, "node.ownerDocument"):
			// No parent frame requires coordinate translation.
		default:
			return fmt.Errorf("unexpected evaluation: %s", p.FunctionDeclaration)
		}
	case runtime.CommandReleaseObject:
		s.released = append(s.released, params.(*runtime.ReleaseObjectParams).ObjectID)
		s.events = append(s.events, "release")
		return s.releaseErr
	case dom.CommandScrollIntoViewIfNeeded:
	case dom.CommandGetBoxModel:
		result.(*dom.GetBoxModelReturns).Model = &dom.BoxModel{Border: dom.Quad{0, 0, 10, 0, 10, 10, 0, 10}}
	default:
		return fmt.Errorf("unexpected CDP command: %s", method)
	}
	return nil
}

func newHandleReleaseFrame(t *testing.T, s *handleReleaseSession) *Frame {
	t.Helper()
	ctx := context.Background()
	logger := log.NewNullLogger()
	manager := NewFrameManager(ctx, nil, nil, NewTimeoutSettings(nil), logger)
	frame := NewFrame(ctx, manager, nil, "frame", logger)
	frame.page = &Page{logger: logger, targetID: "frame", frameManager: manager}
	manager.frames["frame"] = frame
	manager.mainFrame = frame
	execution := &ExecutionContext{ctx: ctx, logger: logger, session: s, frame: frame, id: 1}
	base := func(id runtime.RemoteObjectID) BaseJSHandle {
		return BaseJSHandle{
			ctx: ctx, logger: logger, session: s, execCtx: execution,
			remoteObject: &runtime.RemoteObject{ObjectID: id},
		}
	}
	script := base("injected-script")
	execution.injectedScript = &script
	frame.documentHandle = &ElementHandle{BaseJSHandle: base("document"), frame: frame}
	frame.executionContexts[mainWorld] = execution
	return frame
}

func TestLocatorWaitForReleasesHandle(t *testing.T) {
	t.Parallel()
	waitErr := errors.New("selector evaluation failed")
	cleanupErr := errors.New("release failed")
	for _, tc := range []struct {
		name                  string
		state                 DOMElementState
		missing               bool
		lookupErr, releaseErr error
	}{
		{name: "attached", state: DOMElementStateAttached},
		{name: "visible", state: DOMElementStateVisible},
		{name: "hidden_without_handle", state: DOMElementStateHidden, missing: true},
		{name: "detached_without_handle", state: DOMElementStateDetached, missing: true},
		{name: "lookup_error", lookupErr: waitErr},
		{name: "cleanup_error", releaseErr: cleanupErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &handleReleaseSession{missing: tc.missing, lookupErr: tc.lookupErr, releaseErr: tc.releaseErr}
			frame := newHandleReleaseFrame(t, s)
			hook := test.NewLocal(frame.log.Logger)
			locator := NewLocator(frame.ctx, nil, "#target", frame, frame.log)
			opts := NewFrameWaitForSelectorOptions(time.Second)
			opts.State = tc.state
			err := locator.WaitFor(opts)
			require.ErrorIs(t, err, tc.lookupErr)
			if tc.missing || tc.lookupErr != nil {
				require.Empty(t, s.released)
			} else {
				require.Equal(t, []runtime.RemoteObjectID{"temporary-element"}, s.released)
			}
			if tc.releaseErr != nil {
				entries := hook.AllEntries()
				require.Len(t, entries, 1)
				require.Equal(t, logrus.WarnLevel, entries[0].Level)
				require.Equal(t, "Locator:WaitFor", entries[0].Data["category"])
				require.Contains(t, entries[0].Message, cleanupErr.Error())
			} else {
				require.Empty(t, hook.AllEntries())
			}
		})
	}
}
