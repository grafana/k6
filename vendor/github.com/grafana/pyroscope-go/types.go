package pyroscope

type ProfileType string

// Logger is an interface that library users can use
// It is based on logrus, but much smaller — That's because we don't want library users to have to implement
// all of the logrus's methods
type Logger interface {
	Infof(_ string, _ ...interface{})
	Debugf(_ string, _ ...interface{})
	Errorf(_ string, _ ...interface{})
}

const (
	ProfileCPU           ProfileType = "cpu"
	ProfileInuseObjects  ProfileType = "inuse_objects"
	ProfileAllocObjects  ProfileType = "alloc_objects"
	ProfileInuseSpace    ProfileType = "inuse_space"
	ProfileAllocSpace    ProfileType = "alloc_space"
	ProfileGoroutines    ProfileType = "goroutines"
	ProfileMutexCount    ProfileType = "mutex_count"
	ProfileMutexDuration ProfileType = "mutex_duration"
	ProfileBlockCount    ProfileType = "block_count"
	ProfileBlockDuration ProfileType = "block_duration"
	DefaultSampleRate                = 100
)

var DefaultProfileTypes = []ProfileType{
	ProfileCPU,
	ProfileAllocObjects,
	ProfileAllocSpace,
	ProfileInuseObjects,
	ProfileInuseSpace,
}
