package cmd

import (
	"os"
	"os/exec"

	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
	k6State "go.k6.io/k6/cmd/state"
)

type state struct {
	Options
	gs      *k6State.GlobalState
	cmd     *exec.Cmd
	cleanup func() error
	deps    k6deps.Dependencies
}

func newState(gs *k6State.GlobalState, deps k6deps.Dependencies, opt Options) *state {
	s := new(state)
	s.gs = gs
	s.deps = deps
	s.Options = opt

	return s
}

func (s *state) provision() (string, error) {
	config := k6provider.Config{
		BuildServiceURL:  s.Options.BuildServiceURL,
		BuildServiceAuth: s.Options.BuildServiceToken,
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return "", err
	}

	// TODO: we need a better handle of errors here
	// like (network, auth, etc) and give a better error message
	// to the user
	binary, err := provider.GetBinary(s.gs.Ctx, s.deps)
	if err != nil {
		return "", err
	}

	// TODO: for now we just log the version, but we need to come up with a better UI/UX
	s.gs.Logger.Infof("k6 has been provisioned with the version %q", binary.Dependencies["k6"])

	return binary.Path, nil
}

func (s *state) preRunE() error {
	binPath, err := s.provision()
	if err != nil {
		return err
	}

	s.cleanup = func() error {
		return nil // TODO: implement cleanup
	}

	cmd := exec.CommandContext(s.gs.Ctx, binPath, s.gs.CmdArgs[1:]...) //nolint:gosec

	cmd.Stderr = os.Stderr //nolint:forbidigo
	cmd.Stdout = os.Stdout //nolint:forbidigo
	cmd.Stdin = os.Stdin   //nolint:forbidigo

	s.cmd = cmd

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
