package lib

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenarioStateCurrentStage(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		// this just asserts that the fields are populated as expected
		s1 := ScenarioStage{
			Index:    1,
			Name:     "stage1",
			Duration: time.Second,
		}

		state := ScenarioState{
			Stages: []ScenarioStage{
				{
					Index:    0,
					Duration: 2 * time.Second,
				},
				s1,
			},
		}
		stage, err := state.CurrentStage()
		require.NoError(t, err)
		assert.Equal(t, &s1, stage)
	})

	t.Run("SuccessEdgeCases", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name     string
			stages   []time.Duration
			elapsed  time.Duration // it fakes the elapsed scenario time
			expIndex uint
		}{
			{
				name:     "ZeroTime",
				stages:   []time.Duration{5 * time.Second, 20 * time.Second},
				elapsed:  0,
				expIndex: 0,
			},
			{
				name:     "FirstStage",
				stages:   []time.Duration{5 * time.Second, 20 * time.Second},
				elapsed:  4 * time.Second,
				expIndex: 0,
			},
			{
				name:     "MiddleStage",
				stages:   []time.Duration{5 * time.Second, 20 * time.Second, 10 * time.Second},
				elapsed:  10 * time.Second,
				expIndex: 1,
			},
			{
				name:     "StageUpperLimit",
				stages:   []time.Duration{5 * time.Second, 20 * time.Second, 10 * time.Second},
				elapsed:  25 * time.Second,
				expIndex: 2,
			},
			{
				name:     "OverLatestStage",
				stages:   []time.Duration{5 * time.Second, 20 * time.Second},
				elapsed:  30 * time.Second,
				expIndex: 1,
			},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				stages := func() []ScenarioStage {
					stages := make([]ScenarioStage, 0, len(tc.stages))
					for i, duration := range tc.stages {
						stage := ScenarioStage{
							Index:    uint(i),
							Duration: duration,
						}
						if uint(i) == tc.expIndex {
							stage.Name = tc.name
						}
						stages = append(stages, stage)
					}
					return stages
				}

				state := ScenarioState{
					Stages:    stages(),
					StartTime: time.Now().Add(-tc.elapsed),
				}

				stage, err := state.CurrentStage()
				require.NoError(t, err)
				assert.Equal(t, tc.expIndex, stage.Index)
				assert.Equal(t, tc.name, stage.Name)
			})
		}
	})

	t.Run("ErrorOnEmpty", func(t *testing.T) {
		t.Parallel()
		state := ScenarioState{}
		stage, err := state.CurrentStage()
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "any Stage")
		assert.Nil(t, stage)
	})
}
