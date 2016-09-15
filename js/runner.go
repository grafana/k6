package js

import (
	"context"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
)

type Runner struct {
	Runtime *Runtime
	Exports otto.Value
}

func (r *Runner) NewVU() (lib.VU, error) {
	return &VU{}, nil
}

type VU struct {
	ID int64
}

func (u *VU) RunOnce(ctx context.Context) ([]stats.Sample, error) {
	return nil, nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	return nil
}
