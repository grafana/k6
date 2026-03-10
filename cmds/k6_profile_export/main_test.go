package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
)

func TestAggregateAttributesInclusiveFrames(t *testing.T) {
	t.Parallel()

	fnA := &profile.Function{ID: 1, Name: "a", Filename: "/tmp/script.js"}
	fnB := &profile.Function{ID: 2, Name: "b", Filename: "/tmp/script.js"}

	locB := &profile.Location{ID: 1, Line: []profile.Line{{Function: fnB, Line: 20}}}
	locA := &profile.Location{ID: 2, Line: []profile.Line{{Function: fnA, Line: 10}}}

	pr := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "samples"},
			{Type: "cpu"},
			{Type: "alloc_space"},
			{Type: "alloc_objects"},
		},
		Sample: []*profile.Sample{
			{
				Value:    []int64{1, 1000, 2048, 3},
				Location: []*profile.Location{locB, locA},
			},
		},
	}

	got := aggregate(pr)
	fileRows := got.Files["/tmp/script.js"]
	require.NotNil(t, fileRows)

	require.Equal(t, int64(1000), fileRows[20].CPUNanos)
	require.Equal(t, int64(2048), fileRows[20].AllocSpace)
	require.Equal(t, int64(3), fileRows[20].AllocObjects)
	require.Equal(t, int64(1), fileRows[20].Samples)

	require.Equal(t, int64(1000), fileRows[10].CPUNanos)
	require.Equal(t, int64(2048), fileRows[10].AllocSpace)
	require.Equal(t, int64(3), fileRows[10].AllocObjects)
	require.Equal(t, int64(1), fileRows[10].Samples)
}

func TestAggregateDeduplicatesSameFramePerSample(t *testing.T) {
	t.Parallel()

	fn := &profile.Function{ID: 1, Name: "recurse", Filename: "/tmp/rec.js"}
	loc := &profile.Location{ID: 1, Line: []profile.Line{{Function: fn, Line: 42}}}

	pr := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "samples"},
			{Type: "cpu"},
			{Type: "alloc_space"},
			{Type: "alloc_objects"},
		},
		Sample: []*profile.Sample{
			{
				Value:    []int64{1, 10, 20, 1},
				Location: []*profile.Location{loc, loc, loc},
			},
		},
	}

	got := aggregate(pr)
	row := got.Files["/tmp/rec.js"][42]
	require.Equal(t, int64(10), row.CPUNanos)
	require.Equal(t, int64(20), row.AllocSpace)
	require.Equal(t, int64(1), row.AllocObjects)
	require.Equal(t, int64(1), row.Samples)
}

func TestAggregateAttributesToImporterWhenOnlyImportedFramePresent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	importer := filepath.Join(dir, "main.js")
	dep := filepath.Join(dir, "dep.js")
	require.NoError(t, os.WriteFile(importer, []byte("import './dep.js'\nexport default function(){}\n"), 0o600))
	require.NoError(t, os.WriteFile(dep, []byte("export const value = 42\n"), 0o600))

	fnDep := &profile.Function{ID: 1, Name: "depInit", Filename: dep}
	locDep := &profile.Location{ID: 1, Line: []profile.Line{{Function: fnDep, Line: 1}}}

	pr := &profile.Profile{
		Function: []*profile.Function{fnDep, {ID: 2, Name: "mainInit", Filename: importer}},
		SampleType: []*profile.ValueType{
			{Type: "samples"},
			{Type: "cpu"},
			{Type: "alloc_space"},
			{Type: "alloc_objects"},
		},
		Sample: []*profile.Sample{
			{
				Value:    []int64{1, 200, 300, 4},
				Location: []*profile.Location{locDep},
			},
		},
	}

	got := aggregate(pr)
	impRows := got.Files[normalizeProfilePath(importer)]
	require.NotNil(t, impRows)
	require.Equal(t, int64(200), impRows[1].CPUNanos)
	require.Equal(t, int64(300), impRows[1].AllocSpace)
	require.Equal(t, int64(4), impRows[1].AllocObjects)
	require.Equal(t, int64(1), impRows[1].Samples)
}

func TestAggregateDoesNotDoubleCountImporterWhenCallerFrameExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	importer := filepath.Join(dir, "main.js")
	dep := filepath.Join(dir, "dep.js")
	require.NoError(t, os.WriteFile(importer, []byte("import './dep.js'\nexport default function(){}\n"), 0o600))
	require.NoError(t, os.WriteFile(dep, []byte("export const value = 42\n"), 0o600))

	fnDep := &profile.Function{ID: 1, Name: "depInit", Filename: dep}
	fnImporter := &profile.Function{ID: 2, Name: "mainInit", Filename: importer}
	locDep := &profile.Location{ID: 1, Line: []profile.Line{{Function: fnDep, Line: 1}}}
	locImporter := &profile.Location{ID: 2, Line: []profile.Line{{Function: fnImporter, Line: 1}}}

	pr := &profile.Profile{
		Function: []*profile.Function{fnDep, fnImporter},
		SampleType: []*profile.ValueType{
			{Type: "samples"},
			{Type: "cpu"},
			{Type: "alloc_space"},
			{Type: "alloc_objects"},
		},
		Sample: []*profile.Sample{
			{
				Value:    []int64{1, 200, 300, 4},
				Location: []*profile.Location{locDep, locImporter},
			},
		},
	}

	got := aggregate(pr)
	impRows := got.Files[normalizeProfilePath(importer)]
	require.NotNil(t, impRows)
	require.Equal(t, int64(200), impRows[1].CPUNanos)
	require.Equal(t, int64(300), impRows[1].AllocSpace)
	require.Equal(t, int64(4), impRows[1].AllocObjects)
	require.Equal(t, int64(1), impRows[1].Samples)
}
