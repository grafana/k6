package agent

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/grafana/xk6-disruptor/pkg/runtime/profiler"
)

// FakeProtocolDisruptor implements a fake protocol Disruptor
type FakeProtocolDisruptor struct{}

// Apply implements the Apply method from the protocol Disruptor interface
func (d *FakeProtocolDisruptor) Apply(_ context.Context, duration time.Duration) error {
	time.Sleep(duration)
	return nil
}

func Test_CancelContext(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title    string
		vars     map[string]string
		args     []string
		delay    time.Duration
		err      error
		config   *Config
		expected error
	}{
		{
			title: "Command is not canceled",
			vars:  map[string]string{},
			args:  []string{},
			delay: 0 * time.Second,
			err:   nil,
			config: &Config{
				Profiler: &profiler.Config{},
			},
			expected: nil,
		},
		{
			title: "Command is canceled",
			delay: 5 * time.Second,
			err:   nil,
			vars:  map[string]string{},
			args:  []string{},
			config: &Config{
				Profiler: &profiler.Config{},
			},
			expected: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			env := runtime.NewFakeRuntime(tc.args, tc.vars)

			agent, err := Start(env, tc.config)
			if err != nil {
				t.Fatalf("starting agent: %v", err)
			}

			defer agent.Stop()

			ctx, cancel := context.WithCancel(t.Context())
			go func() {
				time.Sleep(1 * time.Second)
				cancel()
			}()

			disruptor := &FakeProtocolDisruptor{}
			err = agent.ApplyDisruption(ctx, disruptor, tc.delay)
			if !errors.Is(err, tc.expected) {
				t.Errorf("expected %v got %v", tc.err, err)
			}
		})
	}
}

func Test_Signals(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title     string
		args      []string
		vars      map[string]string
		delay     time.Duration
		err       error
		config    *Config
		signal    syscall.Signal
		expectErr bool
	}{
		{
			title: "Command is canceled with interrupt",
			args:  []string{},
			vars:  map[string]string{},
			delay: 5 * time.Second,
			err:   nil,
			config: &Config{
				Profiler: &profiler.Config{},
			},
			signal:    syscall.SIGINT,
			expectErr: true,
		},
		{
			title: "Command is not canceled with interrupt",
			args:  []string{},
			vars:  map[string]string{},
			delay: 5 * time.Second,
			err:   nil,
			config: &Config{
				Profiler: &profiler.Config{},
			},
			signal:    0,
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			env := runtime.NewFakeRuntime(tc.args, tc.vars)

			agent, err := Start(env, tc.config)
			if err != nil {
				t.Fatalf("starting agent: %v", err)
			}

			defer agent.Stop()

			go func() {
				time.Sleep(1 * time.Second)
				if tc.signal != 0 {
					env.FakeSignal.Send(tc.signal)
				}
			}()

			disruptor := &FakeProtocolDisruptor{}
			err = agent.ApplyDisruption(t.Context(), disruptor, tc.delay)
			if tc.expectErr && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		})
	}
}
