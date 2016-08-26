package stats

type Backend interface {
	Submit(batches [][]Sample) error
}
