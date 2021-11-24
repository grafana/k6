package segment_test

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules/k6/segment"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/stats"
)

func TestModuleInstanceSegmentedIndex(t *testing.T) {
	t.Parallel()

	es, err := lib.NewExecutionSegmentFromString("0:1")
	require.NoError(t, err)

	ess, err := lib.NewExecutionSegmentSequenceFromString("0,1")
	require.NoError(t, err)

	state := &lib.State{
		Options: lib.Options{
			ExecutionSegment:         es,
			ExecutionSegmentSequence: &ess,
			SystemTags:               stats.NewSystemTagSet(stats.TagVU),
		},
		Tags:   lib.NewTagMap(nil),
		Logger: testutils.NewLogger(t),
	}

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	ctx := common.WithRuntime(context.Background(), rt)
	ctx = lib.WithState(ctx, state)
	m, ok := segment.New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			InitEnvField: &common.InitEnvironment{},
			CtxField:     ctx,
			StateField:   state,
		},
	).(*segment.ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("segment", m.Exports().Named))

	si, err := rt.RunString(`
		var index = new segment.SegmentedIndex();
		var v = index.next()
		if (v === undefined) {
			throw('v is undefined')
		}
		if (v.scaled !== 1) {
			throw('got unexpected value for scaled')
		}
		if (v.unscaled !== 1) {
			throw('got unexpected value for unscaled')
		}
	`)
	require.NoError(t, err)
	assert.NotNil(t, si)
}

func TestModuleInstanceSharedSegmentedIndex(t *testing.T) {
	t.Parallel()

	es, err := lib.NewExecutionSegmentFromString("0:1")
	require.NoError(t, err)

	ess, err := lib.NewExecutionSegmentSequenceFromString("0,1")
	require.NoError(t, err)

	state := &lib.State{
		Options: lib.Options{
			ExecutionSegment:         es,
			ExecutionSegmentSequence: &ess,
			SystemTags:               stats.NewSystemTagSet(stats.TagVU),
		},
		Tags:   lib.NewTagMap(nil),
		Logger: testutils.NewLogger(t),
	}

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	ctx := common.WithRuntime(context.Background(), rt)
	ctx = lib.WithState(ctx, state)
	m, ok := segment.New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			InitEnvField: &common.InitEnvironment{},
			CtxField:     ctx,
			StateField:   state,
		},
	).(*segment.ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("segment", m.Exports().Named))

	si, err := rt.RunString(`
		var index = new segment.SharedSegmentedIndex('myarr');
		var v = index.next()
		if (v === undefined) {
			throw('v is undefined')
		}
		if (v.scaled !== 1) {
			throw('got unexpected value for scaled')
		}
		if (v.unscaled !== 1) {
			throw('got unexpected value for unscaled')
		}
	`)
	require.NoError(t, err)
	assert.NotNil(t, si)
}
