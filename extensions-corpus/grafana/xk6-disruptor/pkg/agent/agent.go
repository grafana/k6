// Package agent implements functions for injecting faults in a target
package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/grafana/xk6-disruptor/pkg/runtime/profiler"
)

// Config maintains the configuration for the execution of the agent
type Config struct {
	Profiler *profiler.Config
}

// Agent maintains the state required for executing an agent command
type Agent struct {
	env           runtime.Environment
	sc            <-chan os.Signal
	profileCloser io.Closer
}

// Disruptor defines the interface for applying disruptions
type Disruptor interface {
	Apply(context.Context, time.Duration) error
}

// Start creates and starts a new instance of an agent.
// Returned agent is guaranteed to be unique in the environment it is running, and will handle signals sent to the
// process.
// Callers must Stop the returned agent at the end of its lifecycle.
func Start(env runtime.Environment, config *Config) (*Agent, error) {
	a := &Agent{
		env: env,
	}

	if err := a.start(config); err != nil {
		a.Stop() // Stop any initialized component if initialization failed.
		return nil, err
	}

	return a, nil
}

func (a *Agent) start(config *Config) error {
	a.sc = a.env.Signal().Notify(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	acquired, err := a.env.Lock().Acquire()
	if err != nil {
		return fmt.Errorf("could not acquire process lock: %w", err)
	}

	if !acquired {
		return fmt.Errorf("another instance of the agent is already running")
	}

	// start profiler
	a.profileCloser, err = a.env.Profiler().Start(*config.Profiler)
	if err != nil {
		return fmt.Errorf("could not create profiler %w", err)
	}

	return nil
}

// ApplyDisruption applies a disruption to the target
func (a *Agent) ApplyDisruption(ctx context.Context, disruptor Disruptor, duration time.Duration) error {
	// set context for command
	ctx, cancel := context.WithCancel(ctx)

	// execute action goroutine to prevent blocking
	cc := make(chan error)
	go func() {
		cc <- disruptor.Apply(ctx, duration)
		close(cc)
	}()

	defer func() {
		<-cc
	}()

	defer cancel()

	// wait for command completion or cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-cc:
		return err
	case s := <-a.sc:
		return fmt.Errorf("received signal %q", s)
	}
}

// Stop stops a running agent: It releases
func (a *Agent) Stop() {
	a.env.Signal().Reset()
	_ = a.env.Lock().Release()

	if a.profileCloser != nil {
		_ = a.profileCloser.Close()
	}
}
