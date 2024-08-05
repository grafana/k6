package pprof

import (
	"io"
	"runtime"
)

type DeltaMutexProfiler struct {
	m       profMap
	mem     []memMap
	Options ProfileBuilderOptions
}

// PrintCountCycleProfile outputs block profile records (for block or mutex profiles)
// as the pprof-proto format output. Translations from cycle count to time duration
// are done because The proto expects count and time (nanoseconds) instead of count
// and the number of cycles for block, contention profiles.
// Possible 'scaler' functions are scaleBlockProfile and scaleMutexProfile.
func (d *DeltaMutexProfiler) PrintCountCycleProfile(w io.Writer, countName, cycleName string, scaler MutexProfileScaler, records []runtime.BlockProfileRecord) error {
	if d.mem == nil || !d.Options.LazyMapping {
		d.mem = readMapping()
	}
	// Output profile in protobuf form.
	b := newProfileBuilder(w, d.Options, d.mem)
	b.pbValueType(tagProfile_PeriodType, countName, "count")
	b.pb.int64Opt(tagProfile_Period, 1)
	b.pbValueType(tagProfile_SampleType, countName, "count")
	b.pbValueType(tagProfile_SampleType, cycleName, "nanoseconds")

	cpuGHz := float64(runtime_cyclesPerSecond()) / 1e9

	values := []int64{0, 0}
	var locs []uint64
	for _, r := range records {
		count, nanosec := ScaleMutexProfile(scaler, r.Count, float64(r.Cycles)/cpuGHz)
		inanosec := int64(nanosec)

		// do the delta
		entry := d.m.Lookup(r.Stack(), 0)
		values[0] = count - entry.count.v1
		values[1] = inanosec - entry.count.v2
		entry.count.v1 = count
		entry.count.v2 = inanosec

		if values[0] < 0 || values[1] < 0 {
			continue
		}
		if values[0] == 0 && values[1] == 0 {
			continue
		}

		// For count profiles, all stack addresses are
		// return PCs, which is what appendLocsForStack expects.
		locs = b.appendLocsForStack(locs[:0], r.Stack())
		b.pbSample(values, locs, nil)
	}
	b.build()
	return nil
}
