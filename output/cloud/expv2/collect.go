package expv2

import (
	"errors"
	"sync"
	"time"

	"go.k6.io/k6/metrics"
)

type timeBucket struct {
	Time  int64
	Sinks map[metrics.TimeSeries]metricValue
}

// bucketQ is a queue for buffering the aggregated metrics
// that have to be flushed. It is expected to be used concurrently.
type bucketQ struct {
	m       sync.Mutex
	buckets []timeBucket
}

// PopAll returns a slice with all the pushed buckets.
// It returns a nil slice if the queue is empty.
func (q *bucketQ) PopAll() []timeBucket {
	q.m.Lock()
	defer q.m.Unlock()

	if len(q.buckets) < 1 {
		return nil
	}

	// return the enqueued slice and its relative array and allocate a new one
	// using the same capacity.
	b := q.buckets
	q.buckets = make([]timeBucket, 0, len(b))
	return b
}

// Push enqueues a new item in the queue.
func (q *bucketQ) Push(b []timeBucket) {
	if len(b) < 1 {
		return
	}
	q.m.Lock()
	q.buckets = append(q.buckets, b...)
	q.m.Unlock()
}

type collector struct {
	bq      bucketQ
	nowFunc func() time.Time

	aggregationPeriod time.Duration
	waitPeriod        time.Duration

	// we should no longer have to handle metrics that have times long in the past. So instead of a
	// map, we can probably use a simple slice (or even an array!) as a ring buffer to store the
	// aggregation buckets. This should save us a some time, since it would make the lookups and WaitPeriod
	// checks basically O(1). And even if for some reason there are occasional metrics with past times that
	// don't fit in the chosen ring buffer size, we could just send them along to the buffer unaggregated
	timeBuckets map[int64]map[metrics.TimeSeries]metricValue
}

func newCollector(aggrPeriod, waitPeriod time.Duration) (*collector, error) {
	if aggrPeriod == 0 {
		return nil, errors.New("aggregation period is not allowed to be zero")
	}
	if aggrPeriod != aggrPeriod.Truncate(time.Second) {
		return nil, errors.New("aggregation period is not allowed to have sub-second precision")
	}
	if waitPeriod == 0 {
		// TODO: we could simplify the expiring logic
		// just having an internal static logic.
		// Like skip only not closed buckets bucketEnd > now.
		return nil, errors.New("aggregation wait period is not allowed to be zero")
	}
	if waitPeriod != waitPeriod.Truncate(time.Second) {
		return nil, errors.New("aggregation wait period is not allowed to have sub-second precision")
	}
	return &collector{
		bq:                bucketQ{},
		nowFunc:           time.Now,
		timeBuckets:       make(map[int64]map[metrics.TimeSeries]metricValue),
		aggregationPeriod: aggrPeriod,
		waitPeriod:        waitPeriod,
	}, nil
}

// CollectSamples drain the buffer and collect all the samples.
func (c *collector) CollectSamples(containers []metrics.SampleContainer) {
	// Distribute all newly buffered samples into related buckets
	for _, sampleContainer := range containers {
		samples := sampleContainer.GetSamples()

		for i := 0; i < len(samples); i++ {
			c.collectSample(samples[i])
		}
	}
	c.bq.Push(c.expiredBuckets())
}

// DropExpiringDelay drops the waiting time for buckets
// for the expiring checks.
func (c *collector) DropExpiringDelay() {
	c.waitPeriod = 0
}

func (c *collector) collectSample(s metrics.Sample) {
	bucketID := c.bucketID(s.Time)

	// Get or create a time bucket
	bucket, ok := c.timeBuckets[bucketID]
	if !ok {
		bucket = make(map[metrics.TimeSeries]metricValue)
		c.timeBuckets[bucketID] = bucket
	}

	// Get or create the bucket's sinks map per time series
	sink, ok := bucket[s.TimeSeries]
	if !ok {
		sink = newMetricValue(s.Metric.Type)
		bucket[s.TimeSeries] = sink
	}

	sink.Add(s.Value)
}

func (c *collector) expiredBuckets() []timeBucket {
	// Still too recent buckets
	// where we prefer to wait a bit more
	// then, hopefully, we can aggregate more samples before flushing.
	bucketCutoffID := c.bucketCutoffID()

	// Here, it avoids pre-allocation
	// because it expects to be zero for most of the time
	var expired []timeBucket //nolint:prealloc

	// Mark as expired all aggregation buckets older than bucketCutoffID
	for bucketID, seriesSinks := range c.timeBuckets {
		if bucketID > bucketCutoffID {
			continue
		}

		expired = append(expired, timeBucket{
			Time:  c.timeFromBucketID(bucketID),
			Sinks: seriesSinks,
		})
		delete(c.timeBuckets, bucketID)
	}

	return expired
}

func (c *collector) bucketID(t time.Time) int64 {
	return t.UnixNano() / int64(c.aggregationPeriod)
}

func (c *collector) timeFromBucketID(id int64) int64 {
	return id * int64(c.aggregationPeriod)
}

func (c *collector) bucketCutoffID() int64 {
	return c.nowFunc().Add(-c.waitPeriod).UnixNano() / int64(c.aggregationPeriod)
}
