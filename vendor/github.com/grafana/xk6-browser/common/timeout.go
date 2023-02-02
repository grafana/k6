package common

// TimeoutSettings holds information on timeout settings.
type TimeoutSettings struct {
	parent                   *TimeoutSettings
	defaultTimeout           *int64
	defaultNavigationTimeout *int64
}

// NewTimeoutSettings creates a new timeout settings object.
func NewTimeoutSettings(parent *TimeoutSettings) *TimeoutSettings {
	t := &TimeoutSettings{
		parent:                   parent,
		defaultTimeout:           nil,
		defaultNavigationTimeout: nil,
	}
	return t
}

func (t *TimeoutSettings) setDefaultTimeout(timeout int64) {
	t.defaultTimeout = &timeout
}

func (t *TimeoutSettings) setDefaultNavigationTimeout(timeout int64) {
	t.defaultNavigationTimeout = &timeout
}

func (t *TimeoutSettings) navigationTimeout() int64 {
	if t.defaultNavigationTimeout != nil {
		return *t.defaultNavigationTimeout
	}
	if t.defaultTimeout != nil {
		return *t.defaultTimeout
	}
	if t.parent != nil {
		return t.parent.navigationTimeout()
	}
	return int64(DefaultTimeout.Seconds())
}

func (t *TimeoutSettings) timeout() int64 {
	if t.defaultTimeout != nil {
		return *t.defaultTimeout
	}
	if t.parent != nil {
		return t.parent.timeout()
	}
	return int64(DefaultTimeout.Seconds())
}
