package stats

var DefaultRegistry = Registry{}
var DefaultCollector = DefaultRegistry.NewCollector()

func NewCollector() *Collector {
	return DefaultRegistry.NewCollector()
}

func Submit() error {
	return DefaultRegistry.Submit()
}

func Add(s Sample) {
	DefaultCollector.Add(s)
}
