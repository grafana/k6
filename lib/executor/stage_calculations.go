package executor

import (
	"math"
	"time"
)

// stageTransitionCalculator is a state machine (of sorts) that calculates the transtitions between
// stages and gives the next full value produced by the stages. In this explanation we are talking
// about events and in practice those events are either starting of an iteration or starting of a
// new VU, but could really be anything.
//
// The basic idea is that we make a graph with the X axis being time and the Y axis being
// events/s we know that the area of the figure between the graph and the X axis is equal to the
// amount of events done - we multiply time by events per time so we get events ...
// Mathematics :).
//
// Lets look at a simple example - lets say we start with 2 events and the first stage is 5
// seconds to 2 events/s and then we have a second stage for 5 second that goes up to 3 events
// (using small numbers because ... well it is easier :D). This will look something like:
//  ^
// 7|
// 6|
// 5|
// 4|
// 3|       ,-+
// 2|----+-'  |
// 1|    |    |
//  +----+----+---------------------------------->
//  0s   5s   10s
// TODO: bigger and more stages
//
// Now the question is when(where on the graph) does the first event happen? Well in this simple
// case it is easy it will be at 0.5 seconds as we are doing 2 events/s. If we want to know when
// event n will happen we need to calculate n = 2 * x, where x is the time it will happen, so we
// need to calculate x = n/2as we are interested in the time, x.
// So if we just had a constant function for each event n we can calculate n/2 and find out when
// it needs to start.
// As we can see though the graph changes as stages change. But we can calculate how many events
// each stage will have, again it is the area from the start of the stage to it's end and between
// the graph and the X axis. So in this case we know that the first stage will have 10 full events
// in it and no more or less. So we are trying to find out when the 12 event will happen the answer
// will be after the 5th second.
//
// The graph doesn't show this well but we are ramping up linearly (we could possibly add
// other ramping up/down functions later). So at 7.5 seconds for example we should be doing 2.5
// events/s. You could start slicing the graph constantly and in this way to represent the ramping
// up/down as a multiple constant functions, and you will get mostly okayish results. But here is
// where calculus comes into play. Calculus gives us a way of exactly calculate the area for any
// given function and linear ramp up/downs just happen to be pretty easy(actual math prove in
// https://github.com/loadimpact/k6/issues/1299#issuecomment-575661084).
//
// One tricky last point is what happens if stage only completes 9.8 events? Let's say that the
// first stage above was 4.9 seconds long 2 * 4.9 is 9.8, we have 9 events and .8 of an event, what
// do with do with that? Well the 10th even will happen in the next stage (if any) and will happen
// when the are from the start till time x is 0.2 (instead of 1) as 0.2 + 0.8 is 10. So the 12th for
// example will be when the area is 2.2 as 9.8+2.2. So we just carry this around.
//
// So in the end what stageTransitionCalculator is doing is to get formulas which will tell it when
// a given event n in order will happen. It helps itself by knowing that in a given
// stage will do some given amount (the area of the stage) events and if we past that one we
// know we are not in that stage.
//
// The specific implementation here can only go forward and does incorporate
// the striping algorithm from the lib.ExecutionTuple for additional speed up but this could
// possibly be refactored if need for this arises.
type stageTransitionCalculator struct {
	index, dur, to, from, doneSoFar, endCount float64
	stages                                    []Stage
	offsets                                   []int64
	globalIndex, stageIndex                   int
	timeUnit                                  int64
	stageStart                                time.Duration
	toMinusFrom                               float64
}

// nextEvent returns the time for the next event it should be called only after more was called and
// it returned true
func (stc *stageTransitionCalculator) nextEvent() time.Duration {
	t := time.Duration(stc.currentStep()) + stc.stageStart
	stc.updateIndex()
	return t
}

// returns whether there is a next event, must be called before nextEvent is called
func (stc *stageTransitionCalculator) more() bool {
	if stc.index > stc.endCount {
		if !stc.nextStage() {
			return false
		}
	}
	return true
}

func (stc *stageTransitionCalculator) loop(f func(time.Duration)) {
	for ok := true; ok; ok = stc.nextStage() {
		if stc.toMinusFrom == 0 {
			for ; stc.index <= stc.endCount; stc.updateIndex() {
				f(time.Duration(constantForStep(*stc)) + stc.stageStart)
			}
		} else {
			for ; stc.index <= stc.endCount; stc.updateIndex() {
				f(time.Duration(linearRampForStep(*stc)) + stc.stageStart)
			}
		}
	}
}

func newStageTransitionCalculator(
	start int64, offsets []int64, startRate, timeUnit int64, stages []Stage,
) stageTransitionCalculator {
	stc := stageTransitionCalculator{
		from:     float64(startRate) / float64(timeUnit),
		index:    float64(start + 1),
		offsets:  offsets,
		timeUnit: timeUnit, // TODO: test
		stages:   stages,
	}
	stc.updateFromStage()
	return stc
}
func (stc *stageTransitionCalculator) updateIndex() {
	stc.index += float64(stc.offsets[stc.globalIndex%len(stc.offsets)])
	stc.globalIndex++
}

func (stc *stageTransitionCalculator) stageEndCount() float64 {
	if stc.toMinusFrom == 0 {
		return constantEndCount(*stc)
	}
	return linearRampEndCount(*stc)
}

func (stc *stageTransitionCalculator) currentStep() float64 {
	if stc.toMinusFrom == 0 {
		return constantForStep(*stc)
	}
	return linearRampForStep(*stc)
}

func (stc *stageTransitionCalculator) updateFromStage() {
	stage := stc.stages[stc.stageIndex]
	stc.to = float64(stage.Target.ValueOrZero()) / float64(stc.timeUnit)
	stc.dur = float64(time.Duration(stage.Duration.Duration).Nanoseconds())
	stc.toMinusFrom = stc.to - stc.from
	stc.endCount += stc.stageEndCount()
}

func (stc *stageTransitionCalculator) nextStage() bool {
	if stc.stageIndex >= len(stc.stages)-1 {
		return false
	}

	stc.from = stc.to
	stc.doneSoFar = stc.endCount
	stc.stageStart += time.Duration(stc.stages[stc.stageIndex].Duration.Duration)
	stc.stageIndex++
	stc.updateFromStage()

	if stc.index > stc.endCount {
		return stc.nextStage()
	}
	return true
}

func linearRampEndCount(c stageTransitionCalculator) float64 {
	return c.dur * ((c.toMinusFrom)/2 + c.from)
}

func linearRampForStep(c stageTransitionCalculator) float64 {
	// TODO: try to twist this in a way to be able to get iteration (the only changing part)
	// somewhere where it is less in the middle of the equation
	// TODO some of this are calculated constantly and can probably be cached
	// the minus is because we should divede by from-to not to-from ;)
	return -(c.from*c.dur - math.Sqrt(c.dur*(c.from*c.from*c.dur+2*(c.index-c.doneSoFar)*c.toMinusFrom))) / c.toMinusFrom
}

func constantEndCount(c stageTransitionCalculator) float64 {
	return c.dur * c.to
}

func constantForStep(c stageTransitionCalculator) float64 {
	return (c.index - c.doneSoFar) / c.to
}
