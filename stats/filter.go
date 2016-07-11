package stats

type Filter struct {
	exclude map[string]bool
	only    map[string]bool
}

func MakeFilter(exclude, only []string) Filter {
	f := Filter{}

	if len(exclude) > 0 {
		f.exclude = make(map[string]bool)
		for _, stat := range exclude {
			f.exclude[stat] = true
		}
	}
	if len(only) > 0 {
		f.only = make(map[string]bool)
		for _, stat := range only {
			f.only[stat] = true
		}
	}

	return f
}

func (f Filter) Check(s Sample) bool {
	if f.only != nil {
		return f.only[s.Stat.Name]
	}
	return !f.exclude[s.Stat.Name]
}
