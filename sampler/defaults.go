package sampler

var DefaultSampler = New()

func Get(name string) *Metric {
	return DefaultSampler.Get(name)
}

func GetAs(name string, t int) *Metric {
	return DefaultSampler.GetAs(name, t)
}

func Counter(name string) *Metric {
	return DefaultSampler.Counter(name)
}

func Stats(name string) *Metric {
	return DefaultSampler.Stats(name)
}
