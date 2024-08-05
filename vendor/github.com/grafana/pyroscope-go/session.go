package pyroscope

import (
	"bytes"
	"math"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/grafana/pyroscope-go/godeltaprof"
	"github.com/grafana/pyroscope-go/internal/flameql"
	"github.com/grafana/pyroscope-go/upstream"
)

type Session struct {
	// configuration, doesn't change
	upstream      upstream.Upstream
	profileTypes  []ProfileType
	uploadRate    time.Duration
	disableGCRuns bool
	// Deprecated: the field will be removed in future releases.
	DisableAutomaticResets bool

	logger   Logger
	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup
	flushCh  chan *flush

	// these things do change:
	memBuf *bytes.Buffer

	goroutinesBuf *bytes.Buffer
	mutexBuf      *bytes.Buffer
	blockBuf      *bytes.Buffer

	lastGCGeneration uint32
	appName          string
	startTime        time.Time

	deltaBlock *godeltaprof.BlockProfiler
	deltaMutex *godeltaprof.BlockProfiler
	deltaHeap  *godeltaprof.HeapProfiler
	cpu        *cpuProfileCollector
}

type SessionConfig struct {
	Upstream       upstream.Upstream
	Logger         Logger
	AppName        string
	Tags           map[string]string
	ProfilingTypes []ProfileType
	DisableGCRuns  bool
	UploadRate     time.Duration

	// Deprecated: the field will be removed in future releases.
	// Use UploadRate instead.
	DisableAutomaticResets bool
	// Deprecated: the field will be removed in future releases.
	// DisableCumulativeMerge is ignored.
	DisableCumulativeMerge bool
	// Deprecated: the field will be removed in future releases.
	// SampleRate is set to 100 and is not configurable.
	SampleRate uint32
}

type flush struct {
	wg   sync.WaitGroup
	wait bool
}

func NewSession(c SessionConfig) (*Session, error) {
	if c.UploadRate == 0 {
		// For backward compatibility.
		c.UploadRate = 15 * time.Second
	}

	c.Logger.Infof("starting profiling session:")
	c.Logger.Infof("  AppName:        %+v", c.AppName)
	c.Logger.Infof("  Tags:           %+v", c.Tags)
	c.Logger.Infof("  ProfilingTypes: %+v", c.ProfilingTypes)
	c.Logger.Infof("  DisableGCRuns:  %+v", c.DisableGCRuns)
	c.Logger.Infof("  UploadRate:     %+v", c.UploadRate)

	if c.DisableAutomaticResets {
		c.UploadRate = math.MaxInt64
	}

	appName, err := mergeTagsWithAppName(c.AppName, newSessionID(), c.Tags)
	if err != nil {
		return nil, err
	}

	ps := &Session{
		upstream:      c.Upstream,
		appName:       appName,
		profileTypes:  c.ProfilingTypes,
		disableGCRuns: c.DisableGCRuns,
		uploadRate:    c.UploadRate,
		stopCh:        make(chan struct{}),
		flushCh:       make(chan *flush),
		logger:        c.Logger,
		memBuf:        &bytes.Buffer{},
		goroutinesBuf: &bytes.Buffer{},
		mutexBuf:      &bytes.Buffer{},
		blockBuf:      &bytes.Buffer{},

		deltaBlock: godeltaprof.NewBlockProfiler(),
		deltaMutex: godeltaprof.NewMutexProfiler(),
		deltaHeap:  godeltaprof.NewHeapProfiler(),
		cpu:        newCPUProfileCollector(appName, c.Upstream, c.Logger, c.UploadRate),
	}

	return ps, nil
}

// mergeTagsWithAppName validates user input and merges explicitly specified
// tags with tags from app name.
//
// App name may be in the full form including tags (app.name{foo=bar,baz=qux}).
// Returned application name is always short, any tags that were included are
// moved to tags map. When merged with explicitly provided tags (config/CLI),
// last take precedence.
//
// App name may be an empty string. Tags must not contain reserved keys,
// the map is modified in place.
func mergeTagsWithAppName(appName string, sid sessionID, tags map[string]string) (string, error) {
	k, err := flameql.ParseKey(appName)
	if err != nil {
		return "", err
	}
	for tagKey, tagValue := range tags {
		if flameql.IsTagKeyReserved(tagKey) {
			continue
		}
		if err = flameql.ValidateTagKey(tagKey); err != nil {
			return "", err
		}
		k.Add(tagKey, tagValue)
	}
	k.Add(sessionIDLabelName, sid.String())
	return k.Normalized(), nil
}

// revive:disable-next-line:cognitive-complexity complexity is fine
func (ps *Session) takeSnapshots() {
	t := time.NewTicker(ps.uploadRate)
	defer t.Stop()
	for {
		select {
		case endTime := <-t.C:
			ps.reset(ps.startTime, endTime)

		case f := <-ps.flushCh:
			ps.reset(ps.startTime, ps.truncatedTime())
			_ = ps.cpu.Flush()
			ps.upstream.Flush()
			f.wg.Done()

		case <-ps.stopCh:
			if ps.isCPUEnabled() {
				ps.cpu.Stop()
			}
			return
		}
	}
}

func copyBuf(b []byte) []byte {
	r := make([]byte, len(b))
	copy(r, b)
	return r
}

func (ps *Session) Start() error {
	t := ps.truncatedTime()
	ps.reset(t, t)

	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()
		ps.takeSnapshots()
	}()

	if ps.isCPUEnabled() {
		ps.wg.Add(1)
		go func() {
			defer ps.wg.Done()
			ps.cpu.Start()
		}()
	}

	return nil
}

func (ps *Session) isCPUEnabled() bool {
	for _, t := range ps.profileTypes {
		if t == ProfileCPU {
			return true
		}
	}
	return false
}

func (ps *Session) isMemEnabled() bool {
	for _, t := range ps.profileTypes {
		if t == ProfileInuseObjects || t == ProfileAllocObjects || t == ProfileInuseSpace || t == ProfileAllocSpace {
			return true
		}
	}
	return false
}

func (ps *Session) isBlockEnabled() bool {
	for _, t := range ps.profileTypes {
		if t == ProfileBlockCount || t == ProfileBlockDuration {
			return true
		}
	}
	return false
}

func (ps *Session) isMutexEnabled() bool {
	for _, t := range ps.profileTypes {
		if t == ProfileMutexCount || t == ProfileMutexDuration {
			return true
		}
	}
	return false
}

func (ps *Session) isGoroutinesEnabled() bool {
	for _, t := range ps.profileTypes {
		if t == ProfileGoroutines {
			return true
		}
	}
	return false
}

func (ps *Session) reset(startTime, endTime time.Time) {
	ps.logger.Debugf("profiling session reset %s", startTime.String())
	// first reset should not result in an upload
	if !ps.startTime.IsZero() {
		ps.uploadData(startTime, endTime)
	}
	ps.startTime = endTime
}

func (ps *Session) uploadData(startTime, endTime time.Time) {
	if ps.isGoroutinesEnabled() {
		p := pprof.Lookup("goroutine")
		if p != nil {
			p.WriteTo(ps.goroutinesBuf, 0)
			ps.upstream.Upload(&upstream.UploadJob{
				Name:            ps.appName,
				StartTime:       startTime,
				EndTime:         endTime,
				SpyName:         "gospy",
				Units:           "goroutines",
				AggregationType: "average",
				Format:          upstream.FormatPprof,
				Profile:         copyBuf(ps.goroutinesBuf.Bytes()),
				SampleTypeConfig: map[string]*upstream.SampleType{
					"goroutine": {
						DisplayName: "goroutines",
						Units:       "goroutines",
						Aggregation: "average",
					},
				},
			})
			ps.goroutinesBuf.Reset()
		}
	}

	if ps.isBlockEnabled() {
		ps.dumpBlockProfile(startTime, endTime)
	}
	if ps.isMutexEnabled() {
		ps.dumpMutexProfile(startTime, endTime)
	}
	if ps.isMemEnabled() {
		ps.dumpHeapProfile(startTime, endTime)
	}
}

func (ps *Session) dumpHeapProfile(startTime time.Time, endTime time.Time) {
	defer func() {
		if r := recover(); r != nil {
			ps.logger.Errorf("dump heap profiler panic %s", string(debug.Stack()))
		}
	}()
	currentGCGeneration := numGC()
	// sometimes GC doesn't run within 10 seconds
	//   in such cases we force a GC run
	//   users can disable it with disableGCRuns option
	if currentGCGeneration == ps.lastGCGeneration && !ps.disableGCRuns {
		runtime.GC()
		currentGCGeneration = numGC()
	}
	if currentGCGeneration != ps.lastGCGeneration {
		ps.memBuf.Reset()
		err := ps.deltaHeap.Profile(ps.memBuf)
		if err != nil {
			ps.logger.Errorf("failed to dump heap profile: %s", err)
			return
		}
		curMemBytes := copyBuf(ps.memBuf.Bytes())
		job := &upstream.UploadJob{
			Name:             ps.appName,
			StartTime:        startTime,
			EndTime:          endTime,
			SpyName:          "gospy",
			SampleRate:       100,
			Format:           upstream.FormatPprof,
			Profile:          curMemBytes,
			SampleTypeConfig: sampleTypeConfigHeap,
		}
		ps.upstream.Upload(job)
		ps.lastGCGeneration = currentGCGeneration
	}
}

func (ps *Session) dumpMutexProfile(startTime time.Time, endTime time.Time) {
	defer func() {
		if r := recover(); r != nil {
			ps.logger.Errorf("dump mutex profiler panic %s", string(debug.Stack()))
		}
	}()
	ps.mutexBuf.Reset()
	ps.deltaMutex.Profile(ps.mutexBuf)
	curMutexBuf := copyBuf(ps.mutexBuf.Bytes())
	job := &upstream.UploadJob{
		Name:             ps.appName,
		StartTime:        startTime,
		EndTime:          endTime,
		SpyName:          "gospy",
		Format:           upstream.FormatPprof,
		Profile:          curMutexBuf,
		SampleTypeConfig: sampleTypeConfigMutex,
	}
	ps.upstream.Upload(job)
}

func (ps *Session) dumpBlockProfile(startTime time.Time, endTime time.Time) {
	defer func() {
		if r := recover(); r != nil {
			ps.logger.Errorf("dump block profiler panic %s", string(debug.Stack()))
		}
	}()
	ps.blockBuf.Reset()
	ps.deltaBlock.Profile(ps.blockBuf)
	curBlockBuf := copyBuf(ps.blockBuf.Bytes())
	job := &upstream.UploadJob{
		Name:             ps.appName,
		StartTime:        startTime,
		EndTime:          endTime,
		SpyName:          "gospy",
		Format:           upstream.FormatPprof,
		Profile:          curBlockBuf,
		SampleTypeConfig: sampleTypeConfigBlock,
	}
	ps.upstream.Upload(job)
}

func (ps *Session) Stop() {
	ps.stopOnce.Do(func() {
		close(ps.stopCh)
		ps.wg.Wait()
	})
}

func (ps *Session) flush(wait bool) {
	f := &flush{
		wg:   sync.WaitGroup{},
		wait: wait,
	}
	f.wg.Add(1)
	ps.flushCh <- f
	if wait {
		f.wg.Wait()
	}
}

func (ps *Session) truncatedTime() time.Time {
	return time.Now().Truncate(ps.uploadRate)
}

func numGC() uint32 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.NumGC
}
