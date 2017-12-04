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
