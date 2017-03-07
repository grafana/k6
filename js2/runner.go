package js2

import (
	"context"
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
)

type Runner struct {
	Options lib.Options

	defaultGroup *lib.Group
}

func New(src *lib.SourceData, fs afero.Fs) (*Runner, error) {
	defaultGroup, err := lib.NewGroup("", nil)
	if err != nil {
		return nil, err
	}

	return &Runner{
		defaultGroup: defaultGroup,
	}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	vu, err := r.newVU()
	if err != nil {
		return nil, err
	}
	return lib.VU(vu), nil
}

func (r *Runner) newVU() (*VU, error) {
	return &VU{}, nil
}

func (r *Runner) GetDefaultGroup() *lib.Group {
	return r.defaultGroup
}

func (r *Runner) GetOptions() lib.Options {
	return r.Options
}

func (r *Runner) ApplyOptions(opts lib.Options) {
	r.Options = r.Options.Apply(opts)
}

type VU struct {
	VM *goja.Runtime
}

func (u *VU) RunOnce(ctx context.Context) ([]stats.Sample, error) {
	return []stats.Sample{}, nil
}

func (u *VU) Reconfigure(id int64) error {
	return nil
}
