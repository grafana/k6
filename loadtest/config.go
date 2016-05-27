package loadtest

import (
	"errors"
	"gopkg.in/yaml.v2"
	"time"
)

// Configuration type for a state.
type ConfigStage struct {
	Duration interface{} `yaml:"duration"`
	VUs      interface{} `yaml:"vus"`
}

type Config struct {
	Duration string        `yaml:"duration"`
	Script   string        `yaml:"script"`
	URL      string        `yaml:"url"`
	VUs      interface{}   `yaml:"vus"`
	Stages   []ConfigStage `yaml:"stages"`
}

func (conf *Config) ParseYAML(data []byte) (err error) {
	return yaml.Unmarshal(data, conf)
}

func parseVUs(vus interface{}) (VUSpec, error) {
	switch v := vus.(type) {
	case nil:
		return VUSpec{}, nil
	case int:
		return VUSpec{Start: v, End: v}, nil
	case []interface{}:
		switch len(v) {
		case 1:
			v0, ok := v[0].(int)
			if !ok {
				return VUSpec{}, errors.New("Item in VU declaration is not an int")
			}
			return VUSpec{Start: v0, End: v0}, nil
		case 2:
			v0, ok0 := v[0].(int)
			v1, ok1 := v[1].(int)
			if !ok0 || !ok1 {
				return VUSpec{}, errors.New("Item in VU declaration is not an int")
			}
			return VUSpec{Start: v0, End: v1}, nil
		default:
			return VUSpec{}, errors.New("Wrong number of values in [start, end] VU ramp")
		}
	default:
		return VUSpec{}, errors.New("VUs must be either a single int or [start, end]")
	}
}

func (c *Config) Compile() (t LoadTest, err error) {
	// Script/URL
	t.Script = c.Script
	t.URL = c.URL
	if t.Script == "" && t.URL == "" {
		return t, errors.New("Script or URL must be specified")
	}

	// Root VU definitions
	rootVUs, err := parseVUs(c.VUs)
	if err != nil {
		return t, err
	}

	// Duration
	rootDurationS := c.Duration
	if rootDurationS == "" {
		rootDurationS = "10s"
	}
	rootDuration, err := time.ParseDuration(rootDurationS)
	if err != nil {
		return t, err
	}

	// Stages
	if len(c.Stages) > 0 {
		// Figure out the scale for flexible durations
		totalFluidDuration := 0
		totalFixedDuration := time.Duration(0)
		for i := 0; i < len(c.Stages); i++ {
			switch v := c.Stages[i].Duration.(type) {
			case int:
				totalFluidDuration += v
			case string:
				duration, err := time.ParseDuration(v)
				if err != nil {
					return t, err
				}
				totalFixedDuration += duration
			}
		}

		// Make sure the fixed segments don't exceed the test length
		available := time.Duration(rootDuration.Nanoseconds() - totalFixedDuration.Nanoseconds())
		if available.Nanoseconds() < 0 {
			return t, errors.New("Fixed stages are exceeding the test duration")
		}

		// Compile stage definitions
		for i := 0; i < len(c.Stages); i++ {
			cStage := &c.Stages[i]
			stage := Stage{}

			// Stage duration
			switch v := cStage.Duration.(type) {
			case int:
				claim := float64(v) / float64(totalFluidDuration)
				stage.Duration = time.Duration(available.Seconds()*claim) * time.Second
			case string:
				stage.Duration, err = time.ParseDuration(v)
			}
			if err != nil {
				return t, err
			}

			// VU curve
			stage.VUs, err = parseVUs(cStage.VUs)
			if err != nil {
				return t, err
			}
			if stage.VUs.Start == 0 && stage.VUs.End == 0 {
				if i > 0 {
					stage.VUs = VUSpec{
						Start: t.Stages[i-1].VUs.End,
						End:   t.Stages[i-1].VUs.End,
					}
				} else {
					stage.VUs = rootVUs
				}
			}

			t.Stages = append(t.Stages, stage)
		}
	} else {
		// Create an implicit, full-duration stage
		t.Stages = []Stage{
			Stage{
				Duration: rootDuration,
				VUs:      rootVUs,
			},
		}
	}

	return t, nil
}
