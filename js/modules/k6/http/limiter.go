package http

type SlotLimiter struct {
	ch chan struct{}
}

func NewSlotLimiter(slots int) SlotLimiter {
	if slots <= 0 {
		return SlotLimiter{nil}
	}

	ch := make(chan struct{}, slots)
	for i := 0; i < slots; i++ {
		ch <- struct{}{}
	}
	return SlotLimiter{ch}
}

func (l *SlotLimiter) Begin() {
	if l.ch != nil {
		<-l.ch
	}
}

func (l *SlotLimiter) End() {
	if l.ch != nil {
		l.ch <- struct{}{}
	}
}

type MultiSlotLimiter struct {
	m     map[string]*SlotLimiter
	slots int
}

func NewMultiSlotLimiter(slots int) MultiSlotLimiter {
	return MultiSlotLimiter{make(map[string]*SlotLimiter), slots}
}

func (l *MultiSlotLimiter) Slot(s string) *SlotLimiter {
	if l.slots == 0 {
		return nil
	}
	ll, ok := l.m[s]
	if !ok {
		tmp := NewSlotLimiter(l.slots)
		ll = &tmp
		l.m[s] = ll
	}
	return ll
}
