/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package output

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/stats"
)

func TestSampleBufferBasics(t *testing.T) {
	t.Parallel()
	single := stats.Sample{
		Time:   time.Now(),
		Metric: stats.New("my_metric", stats.Rate),
		Value:  float64(123),
		Tags:   stats.NewSampleTags(map[string]string{"tag1": "val1"}),
	}
	connected := stats.ConnectedSamples{Samples: []stats.Sample{single, single}, Time: single.Time}
	buffer := SampleBuffer{}

	assert.Empty(t, buffer.GetBufferedSamples())
	buffer.AddMetricSamples([]stats.SampleContainer{single, single})
	buffer.AddMetricSamples([]stats.SampleContainer{single, connected, single})
	assert.Equal(t, []stats.SampleContainer{single, single, single, connected, single}, buffer.GetBufferedSamples())
	assert.Empty(t, buffer.GetBufferedSamples())

	// Verify some internals
	assert.Equal(t, cap(buffer.buffer), 5)
	buffer.AddMetricSamples([]stats.SampleContainer{single, connected})
	buffer.AddMetricSamples(nil)
	buffer.AddMetricSamples([]stats.SampleContainer{})
	buffer.AddMetricSamples([]stats.SampleContainer{single})
	assert.Equal(t, []stats.SampleContainer{single, connected, single}, buffer.GetBufferedSamples())
	assert.Equal(t, cap(buffer.buffer), 4)
	buffer.AddMetricSamples([]stats.SampleContainer{single})
	assert.Equal(t, []stats.SampleContainer{single}, buffer.GetBufferedSamples())
	assert.Equal(t, cap(buffer.buffer), 3)
	assert.Empty(t, buffer.GetBufferedSamples())
}

func TestSampleBufferConcurrently(t *testing.T) {
	t.Parallel()

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed)) //nolint:gosec
	t.Logf("Random source seeded with %d\n", seed)

	producersCount := 50 + r.Intn(50)
	sampleCount := 10 + r.Intn(10)
	sleepModifier := 10 + r.Intn(10)
	buffer := SampleBuffer{}

	wg := make(chan struct{})
	fillBuffer := func() {
		for i := 0; i < sampleCount; i++ {
			buffer.AddMetricSamples([]stats.SampleContainer{stats.Sample{
				Time:   time.Unix(1562324644, 0),
				Metric: stats.New("my_metric", stats.Gauge),
				Value:  float64(i),
				Tags:   stats.NewSampleTags(map[string]string{"tag1": "val1"}),
			}})
			time.Sleep(time.Duration(i*sleepModifier) * time.Microsecond)
		}
		wg <- struct{}{}
	}
	for i := 0; i < producersCount; i++ {
		go fillBuffer()
	}

	timer := time.NewTicker(5 * time.Millisecond)
	timeout := time.After(5 * time.Second)
	defer timer.Stop()
	readSamples := make([]stats.SampleContainer, 0, sampleCount*producersCount)
	finishedProducers := 0
loop:
	for {
		select {
		case <-timer.C:
			readSamples = append(readSamples, buffer.GetBufferedSamples()...)
		case <-wg:
			finishedProducers++
			if finishedProducers == producersCount {
				readSamples = append(readSamples, buffer.GetBufferedSamples()...)
				break loop
			}
		case <-timeout:
			t.Fatalf("test timed out")
		}
	}
	assert.Equal(t, sampleCount*producersCount, len(readSamples))
	for _, s := range readSamples {
		require.NotNil(t, s)
		ss := s.GetSamples()
		require.Len(t, ss, 1)
		assert.Equal(t, "my_metric", ss[0].Metric.Name)
	}
}

func TestPeriodicFlusherBasics(t *testing.T) {
	t.Parallel()

	f, err := NewPeriodicFlusher(-1*time.Second, func() {})
	assert.Error(t, err)
	assert.Nil(t, f)
	f, err = NewPeriodicFlusher(0, func() {})
	assert.Error(t, err)
	assert.Nil(t, f)

	count := 0
	wg := &sync.WaitGroup{}
	wg.Add(1)
	f, err = NewPeriodicFlusher(100*time.Millisecond, func() {
		count++
		if count == 2 {
			wg.Done()
		}
	})
	assert.NotNil(t, f)
	assert.Nil(t, err)
	wg.Wait()
	f.Stop()
	assert.Equal(t, 3, count)
}

func TestPeriodicFlusherConcurrency(t *testing.T) {
	t.Parallel()

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed)) //nolint:gosec
	randStops := 10 + r.Intn(10)
	t.Logf("Random source seeded with %d\n", seed)

	count := 0
	wg := &sync.WaitGroup{}
	wg.Add(1)
	f, err := NewPeriodicFlusher(1000*time.Microsecond, func() {
		// Sleep intentionally may be longer than the flush period. Also, this
		// should never happen concurrently, so it's intentionally not locked.
		time.Sleep(time.Duration(700+r.Intn(1000)) * time.Microsecond)
		count++
		if count == 100 {
			wg.Done()
		}
	})
	assert.NotNil(t, f)
	assert.Nil(t, err)
	wg.Wait()

	stopWG := &sync.WaitGroup{}
	stopWG.Add(randStops)
	for i := 0; i < randStops; i++ {
		go func() {
			f.Stop()
			stopWG.Done()
		}()
	}
	stopWG.Wait()
	assert.True(t, count >= 101) // due to the short intervals, we might not get exactly 101
}
