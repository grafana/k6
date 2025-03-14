package cmd

import (
	"os"
	"os/exec"

	"github.com/grafana/k6deps"
	k6State "go.k6.io/k6/cmd/state"
)

type state struct {
	Options
	gs      *k6State.GlobalState
	cmd     *exec.Cmd
	cleanup func() error
	deps    k6deps.Dependencies
}

func newState(gs *k6State.GlobalState, deps k6deps.Dependencies, opt *Options) *state {
	s := new(state)
	s.gs = gs
	s.deps = deps
	s.Options = *opt

	return s
}

func (s *state) preRunE() error {
	cmd, cleanup, err := Command(s.gs.Ctx, s.gs.CmdArgs[1:], s.deps, &s.Options)
	if err != nil {
		return err
	}

	cmd.Stderr = os.Stderr //nolint:forbidigo
	cmd.Stdout = os.Stdout //nolint:forbidigo
	cmd.Stdin = os.Stdin   //nolint:forbidigo

	s.cmd = cmd
	s.cleanup = cleanup

	return nil
}

func (s *state) runE() error {
	var err error

	// FIXME: I think this code is not setting the error to the cleanup function (pablochacin)
	defer func() {
		e := s.cleanup()
		if err == nil {
			err = e
		}
	}()

	err = s.cmd.Run()

	return err
}
