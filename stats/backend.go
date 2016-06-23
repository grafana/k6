package stats

type Backend interface {
	Submit(batches [][]Point) error
}
