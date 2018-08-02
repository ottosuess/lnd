// +build debug

package ticker

import (
	"sync"
	"sync/atomic"
	"time"
)

// Mock implements the htlcswitch.Ticker interface, and provides a method of
// force-feeding ticks, even while paused.
type Mock struct {
	isActive uint32 // used atomically

	Force chan time.Time

	ticker <-chan time.Time
	ticks  chan time.Time

	wg   sync.WaitGroup
	quit chan struct{}
}

func MockNew(interval time.Duration) *Mock {
	m := &Mock{
		ticker: time.NewTicker(interval).C,
		Force:  make(chan time.Time),
		ticks:  make(chan time.Time),
		quit:   make(chan struct{}),
	}

	// Proxy the real ticks to our ticks channel if we are active, and also
	// allow force feeding ticks regardless of isActive state.
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			var t time.Time
			select {
			case t = <-m.ticker:
				if atomic.LoadUint32(&m.isActive) == 0 {
					continue
				}
			case t = <-m.Force:
			case <-m.quit:
				return
			}

			select {
			case m.ticks <- t:
			case <-m.quit:
				return
			}
		}
	}()

	return m
}

// NOTE: Part of the Ticker interface.
func (m *Mock) Ticks() <-chan time.Time {
	return m.ticks
}

// NOTE: Part of the Ticker interface.
func (m *Mock) IsActive() bool {
	return atomic.LoadUint32(&m.isActive) == 1
}

// NOTE: Part of the Ticker interface.
func (m *Mock) Resume() {
	atomic.StoreUint32(&m.isActive, 1)
}

// NOTE: Part of the Ticker interface.
func (m *Mock) Pause() {
	atomic.StoreUint32(&m.isActive, 0)
}

// NOTE: Part of the Ticker interface.
func (m *Mock) Stop() {
	m.Pause()
	close(m.quit)
	m.wg.Wait()
}
