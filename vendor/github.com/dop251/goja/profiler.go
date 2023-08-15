package goja

import (
	"errors"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/pprof/profile"
)

const profInterval = 10 * time.Millisecond
const profMaxStackDepth = 64

const (
	profReqNone int32 = iota
	profReqDoSample
	profReqSampleReady
	profReqStop
)

type _globalProfiler struct {
	p profiler
	w io.Writer

	enabled int32
}

var globalProfiler _globalProfiler

type profTracker struct {
	req, finished int32
	start, stop   time.Time
	numFrames     int
	frames        [profMaxStackDepth]StackFrame
}

type profiler struct {
	mu       sync.Mutex
	trackers []*profTracker
	buf      *profBuffer
	running  bool
}

type profFunc struct {
	f    profile.Function
	locs map[int32]*profile.Location
}

type profSampleNode struct {
	loc      *profile.Location
	sample   *profile.Sample
	parent   *profSampleNode
	children map[*profile.Location]*profSampleNode
}

type profBuffer struct {
	funcs map[*Program]*profFunc
	root  profSampleNode
}

func (pb *profBuffer) addSample(pt *profTracker) {
	sampleFrames := pt.frames[:pt.numFrames]
	n := &pb.root
	for j := len(sampleFrames) - 1; j >= 0; j-- {
		frame := sampleFrames[j]
		if frame.prg == nil {
			continue
		}
		var f *profFunc
		if f = pb.funcs[frame.prg]; f == nil {
			f = &profFunc{
				locs: make(map[int32]*profile.Location),
			}
			if pb.funcs == nil {
				pb.funcs = make(map[*Program]*profFunc)
			}
			pb.funcs[frame.prg] = f
		}
		var loc *profile.Location
		if loc = f.locs[int32(frame.pc)]; loc == nil {
			loc = &profile.Location{}
			f.locs[int32(frame.pc)] = loc
		}
		if nn := n.children[loc]; nn == nil {
			if n.children == nil {
				n.children = make(map[*profile.Location]*profSampleNode, 1)
			}
			nn = &profSampleNode{
				parent: n,
				loc:    loc,
			}
			n.children[loc] = nn
			n = nn
		} else {
			n = nn
		}
	}
	smpl := n.sample
	if smpl == nil {
		locs := make([]*profile.Location, 0, len(sampleFrames))
		for n1 := n; n1.loc != nil; n1 = n1.parent {
			locs = append(locs, n1.loc)
		}
		smpl = &profile.Sample{
			Location: locs,
			Value:    make([]int64, 2),
		}
		n.sample = smpl
	}
	smpl.Value[0]++
	smpl.Value[1] += int64(pt.stop.Sub(pt.start))
}

func (pb *profBuffer) profile() *profile.Profile {
	pr := profile.Profile{}
	pr.SampleType = []*profile.ValueType{
		{Type: "samples", Unit: "count"},
		{Type: "cpu", Unit: "nanoseconds"},
	}
	pr.PeriodType = pr.SampleType[1]
	pr.Period = int64(profInterval)
	mapping := &profile.Mapping{
		ID:   1,
		File: "[ECMAScript code]",
	}
	pr.Mapping = make([]*profile.Mapping, 1, len(pb.funcs)+1)
	pr.Mapping[0] = mapping

	pr.Function = make([]*profile.Function, 0, len(pb.funcs))
	funcNames := make(map[string]struct{})
	var funcId, locId uint64
	for prg, f := range pb.funcs {
		fileName := prg.src.Name()
		funcId++
		f.f.ID = funcId
		f.f.Filename = fileName
		var funcName string
		if prg.funcName != "" {
			funcName = prg.funcName.String()
		} else {
			funcName = "<anonymous>"
		}
		// Make sure the function name is unique, otherwise the graph display merges them into one node, even
		// if they are in different mappings.
		if _, exists := funcNames[funcName]; exists {
			funcName += "." + strconv.FormatUint(f.f.ID, 10)
		} else {
			funcNames[funcName] = struct{}{}
		}
		f.f.Name = funcName
		pr.Function = append(pr.Function, &f.f)
		for pc, loc := range f.locs {
			locId++
			loc.ID = locId
			pos := prg.src.Position(prg.sourceOffset(int(pc)))
			loc.Line = []profile.Line{
				{
					Function: &f.f,
					Line:     int64(pos.Line),
				},
			}

			loc.Mapping = mapping
			pr.Location = append(pr.Location, loc)
		}
	}
	pb.addSamples(&pr, &pb.root)
	return &pr
}

func (pb *profBuffer) addSamples(p *profile.Profile, n *profSampleNode) {
	if n.sample != nil {
		p.Sample = append(p.Sample, n.sample)
	}
	for _, child := range n.children {
		pb.addSamples(p, child)
	}
}

func (p *profiler) run() {
	ticker := time.NewTicker(profInterval)
	counter := 0

	for ts := range ticker.C {
		p.mu.Lock()
		left := len(p.trackers)
		if left == 0 {
			break
		}
		for {
			// This loop runs until either one of the VMs is signalled or all of the VMs are scanned and found
			// busy or deleted.
			if counter >= len(p.trackers) {
				counter = 0
			}
			tracker := p.trackers[counter]
			req := atomic.LoadInt32(&tracker.req)
			if req == profReqSampleReady {
				p.buf.addSample(tracker)
			}
			if atomic.LoadInt32(&tracker.finished) != 0 {
				p.trackers[counter] = p.trackers[len(p.trackers)-1]
				p.trackers[len(p.trackers)-1] = nil
				p.trackers = p.trackers[:len(p.trackers)-1]
			} else {
				counter++
				if req != profReqDoSample {
					// signal the VM to take a sample
					tracker.start = ts
					atomic.StoreInt32(&tracker.req, profReqDoSample)
					break
				}
			}
			left--
			if left <= 0 {
				// all VMs are busy
				break
			}
		}
		p.mu.Unlock()
	}
	ticker.Stop()
	p.running = false
	p.mu.Unlock()
}

func (p *profiler) registerVm() *profTracker {
	pt := new(profTracker)
	p.mu.Lock()
	if p.buf != nil {
		p.trackers = append(p.trackers, pt)
		if !p.running {
			go p.run()
			p.running = true
		}
	} else {
		pt.req = profReqStop
	}
	p.mu.Unlock()
	return pt
}

func (p *profiler) start() error {
	p.mu.Lock()
	if p.buf != nil {
		p.mu.Unlock()
		return errors.New("profiler is already active")
	}
	p.buf = new(profBuffer)
	p.mu.Unlock()
	return nil
}

func (p *profiler) stop() *profile.Profile {
	p.mu.Lock()
	trackers, buf := p.trackers, p.buf
	p.trackers, p.buf = nil, nil
	p.mu.Unlock()
	if buf != nil {
		k := 0
		for i, tracker := range trackers {
			req := atomic.LoadInt32(&tracker.req)
			if req == profReqSampleReady {
				buf.addSample(tracker)
			} else if req == profReqDoSample {
				// In case the VM is requested to do a sample, there is a small chance of a race
				// where we set profReqStop in between the read and the write, so that the req
				// ends up being set to profReqSampleReady. It's no such a big deal if we do nothing,
				// it just means the VM remains in tracing mode until it finishes the current run,
				// but we do an extra cleanup step later just in case.
				if i != k {
					trackers[k] = trackers[i]
				}
				k++
			}
			atomic.StoreInt32(&tracker.req, profReqStop)
		}

		if k > 0 {
			trackers = trackers[:k]
			go func() {
				// Make sure all VMs are requested to stop tracing.
				for {
					k := 0
					for i, tracker := range trackers {
						req := atomic.LoadInt32(&tracker.req)
						if req != profReqStop {
							atomic.StoreInt32(&tracker.req, profReqStop)
							if i != k {
								trackers[k] = trackers[i]
							}
							k++
						}
					}

					if k == 0 {
						return
					}
					trackers = trackers[:k]
					time.Sleep(100 * time.Millisecond)
				}
			}()
		}
		return buf.profile()
	}
	return nil
}

/*
StartProfile enables execution time profiling for all Runtimes within the current process.
This works similar to pprof.StartCPUProfile and produces the same format which can be consumed by `go tool pprof`.
There are, however, a few notable differences. Firstly, it's not a CPU profile, rather "execution time" profile.
It measures the time the VM spends executing an instruction. If this instruction happens to be a call to a
blocking Go function, the waiting time will be measured. Secondly, the 'cpu' sample isn't simply `count*period`,
it's the time interval between when sampling was requested and when the instruction has finished. If a VM is still
executing the same instruction when the time comes for the next sample, the sampling is skipped (i.e. `count` doesn't
grow).

If there are multiple functions with the same name, their names get a '.N' suffix, where N is a unique number,
because otherwise the graph view merges them together (even if they are in different mappings). This includes
"<anonymous>" functions.

The sampling period is set to 10ms.

It returns an error if profiling is already active.
*/
func StartProfile(w io.Writer) error {
	err := globalProfiler.p.start()
	if err != nil {
		return err
	}
	globalProfiler.w = w
	atomic.StoreInt32(&globalProfiler.enabled, 1)
	return nil
}

/*
StopProfile stops the current profile initiated by StartProfile, if any.
*/
func StopProfile() {
	atomic.StoreInt32(&globalProfiler.enabled, 0)
	pr := globalProfiler.p.stop()
	if pr != nil {
		_ = pr.Write(globalProfiler.w)
	}
	globalProfiler.w = nil
}
