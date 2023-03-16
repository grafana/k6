package platform

const (
	// CpuFeatureSSE3 is the flag to query CpuFeatureFlags.Has for SSEv3 capabilities
	CpuFeatureSSE3 = uint64(1)
	// CpuFeatureSSE4_1 is the flag to query CpuFeatureFlags.Has for SSEv4.1 capabilities
	CpuFeatureSSE4_1 = uint64(1) << 19
	// CpuFeatureSSE4_2 is the flag to query CpuFeatureFlags.Has for SSEv4.2 capabilities
	CpuFeatureSSE4_2 = uint64(1) << 20
)

const (
	// CpuExtraFeatureABM is the flag to query CpuFeatureFlags.HasExtra for Advanced Bit Manipulation capabilities (e.g. LZCNT)
	CpuExtraFeatureABM = uint64(1) << 5
)

// CpuFeatures exposes the capabilities for this CPU, queried via the Has, HasExtra methods
var CpuFeatures CpuFeatureFlags = loadCpuFeatureFlags()

// CpuFeatureFlags exposes methods for querying CPU capabilities
type CpuFeatureFlags interface {
	// Has returns true when the specified flag (represented as uint64) is supported
	Has(cpuFeature uint64) bool
	// HasExtra returns true when the specified extraFlag (represented as uint64) is supported
	HasExtra(cpuFeature uint64) bool
}

// cpuFeatureFlags implements CpuFeatureFlags interface
type cpuFeatureFlags struct {
	flags      uint64
	extraFlags uint64
}

// cpuid exposes the CPUID instruction to the Go layer (https://www.amd.com/system/files/TechDocs/25481.pdf)
// implemented in impl_amd64.s
func cpuid(arg1, arg2 uint32) (eax, ebx, ecx, edx uint32)

// cpuidAsBitmap combines the result of invoking cpuid to uint64 bitmap
func cpuidAsBitmap(arg1, arg2 uint32) uint64 {
	_ /* eax */, _ /* ebx */, ecx, edx := cpuid(arg1, arg2)
	return (uint64(edx) << 32) | uint64(ecx)
}

// loadStandardRange load flags from the standard range, panics otherwise
func loadStandardRange(id uint32) uint64 {
	// ensure that the id is in the valid range, returned by cpuid(0,0)
	maxRange, _, _, _ := cpuid(0, 0)
	if id > maxRange {
		panic("cannot query standard CPU flags")
	}
	return cpuidAsBitmap(id, 0)
}

// loadStandardRange load flags from the extended range, panics otherwise
func loadExtendedRange(id uint32) uint64 {
	// ensure that the id is in the valid range, returned by cpuid(0x80000000,0)
	maxRange, _, _, _ := cpuid(0x80000000, 0)
	if id > maxRange {
		panic("cannot query extended CPU flags")
	}
	return cpuidAsBitmap(id, 0)
}

func loadCpuFeatureFlags() CpuFeatureFlags {
	return &cpuFeatureFlags{
		flags:      loadStandardRange(1),
		extraFlags: loadExtendedRange(0x80000001),
	}
}

// Has implements the same method on the CpuFeatureFlags interface
func (f *cpuFeatureFlags) Has(cpuFeature uint64) bool {
	return (f.flags & cpuFeature) != 0
}

// HasExtra implements the same method on the CpuFeatureFlags interface
func (f *cpuFeatureFlags) HasExtra(cpuFeature uint64) bool {
	return (f.extraFlags & cpuFeature) != 0
}
