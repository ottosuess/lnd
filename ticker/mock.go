// +build debug

package ticker

import (
	"time"
)

// Mock implements the htlcswitch.Ticker interface, and provides a method of
// force-feeding ticks, even while paused.
type Mock struct {
	Force chan time.Time
}

func MockNew() *Mock {
	m := &Mock{
		Force: make(chan time.Time),
	}

	return m
}

// NOTE: Part of the Ticker interface.
func (m *Mock) Ticks() <-chan time.Time {
	return m.Force
}

// NOTE: Part of the Ticker interface.
func (m *Mock) Start() {
}

// NOTE: Part of the Ticker interface.
func (m *Mock) Stop() {
}
