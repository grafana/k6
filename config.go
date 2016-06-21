package main

import (
	"errors"
	"github.com/loadimpact/speedboat/lib"
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

func parseVUs(vus interface{}) (int, int, error) {
	switch v := vus.(type) {
	case int:
		return v, v, nil
	case []interface{}:
		switch len(v) {
		case 1:
			n, ok := v[0].(int)
			if !ok {
				return 0, 0, errors.New("VU counts must be integers")
			}
			return n, n, nil
		case 2:
			n1, ok1 := v[0].(int)
			n2, ok2 := v[1].(int)
			if !ok1 || !ok2 {
				return 0, 0, errors.New("VU counts must be integers")
			}
			return n1, n2, nil
		default:
			return 0, 0, errors.New("Only one or two VU steps allowed per stage")
		}
	case nil:
		return 0, 0, nil
	default:
		return 0, 0, errors.New("VUs must be either an integer or [integer, integer]")
	}
}

func (c *Config) MakeTest() (t lib.Test, err error) {
	t.Script = c.Script
	t.URL = c.URL
	if t.Script == "" && t.URL == "" {
		return t, errors.New("Neither script nor URL specified")
	}

	fullDuration := 10 * time.Second
	if c.Duration != "" {
		fullDuration, err = time.ParseDuration(c.Duration)
		if err != nil {
			return t, err
		}
	}

	if len(c.Stages) > 0 {
		var totalFluid int
		var totalFixed time.Duration

		for _, stage := range c.Stages {
			tStage := lib.TestStage{}

			switch v := stage.Duration.(type) {
			case int:
				totalFluid += v
			case string:
				dur, err := time.ParseDuration(v)
				if err != nil {
					return t, err
				}
				tStage.Duration = dur
				totalFixed += dur
			default:
				return t, errors.New("Stage durations must be integers or strings")
			}

			start, end, err := parseVUs(stage.VUs)
			if err != nil {
				return t, err
			}
			tStage.StartVUs = start
			tStage.EndVUs = end

			t.Stages = append(t.Stages, tStage)
		}

		if totalFixed > fullDuration {
			if totalFluid == 0 {
				fullDuration = totalFixed
			} else {
				return t, errors.New("Stages exceed test duration")
			}
		}

		remainder := fullDuration - totalFixed
		if remainder > 0 {
			for i, stage := range c.Stages {
				chunk, ok := stage.Duration.(int)
				if !ok {
					continue
				}
				t.Stages[i].Duration = time.Duration(chunk) / remainder
			}
		}
	} else {
		start, end, err := parseVUs(c.VUs)
		if err != nil {
			return t, err
		}

		t.Stages = []lib.TestStage{
			lib.TestStage{Duration: fullDuration, StartVUs: start, EndVUs: end},
		}
	}

	return t, nil
}
