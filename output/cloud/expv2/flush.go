package expv2

import "context"

type noopFlusher struct {
	referenceID string
	bq          *bucketQ

	// client      MetricsClient
}

func (f *noopFlusher) Flush(ctx context.Context) error {
	// drain the buffer
	_ = f.bq.PopAll()
	return nil
}
