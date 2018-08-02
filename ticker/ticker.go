package ticker

import "time"

// Ticker defines a resumable ticker interface, whose activity can be toggled to
// free up resources during periods of inactivity.
//
// Example of resuming ticker:
//
//   ticker.Resume() // can remove to keep inactive at first
//   defer ticker.Stop()
//   for {
//     select {
//       case <-ticker.Tick():
//         if shouldGoInactive {
//           ticker.Pause()
//           continue
//         }
//         ...
//
//       case <-otherEvent:
//         ...
//         if shouldGoActive && !ticker.IsActive() {
//           ticker.Resume()
//         }
//     }
//
// NOTE: ONE DOES NOT SIMPLY assume that Tickers are safe for concurrent access.
type Ticker interface {
	// Ticks returns a read-only channel delivering ticks according to a
	// prescribed interval. The value returned does not need to be the same
	// channel, and may be nil.
	//
	// NOTE: Callers should assume that reads from Ticks() are stale after
	// any invocations of Resume, Pause, or Stop.
	Ticks() <-chan time.Time

	// Resume starts or resumes the underlying ticker, such that Ticks()
	// will fire at regular intervals. After calling Resume, IsActive()
	// should return true until receiving a call to Pause or Stop.
	Start()

	// Pause suspends the underlying ticker, such that Ticks() stops
	// signaling at regular intervals. After calling Pause, IsActive()
	// should return false until receiving an invocation of Resume.
	//
	// NOTE: It MUST be safe to call Pause at any time, and more than once
	// successively.
	Stop()
}

// ticker is the production implementation of the resumable Ticker interface.
// This allows various components of the htlcswitch to toggle their need for
// tick events, which may vary depending on system load.
type ticker struct {
	// interval is the desired duration between ticks when active.
	interval time.Duration

	// ticker is the ephemeral, underlying time.Ticker. We keep a reference
	// to this ticker so that it can be stopped and cleaned up on Pause or
	// Stop.
	ticker *time.Ticker
}

// New returns a new ticker that signals with the given interval
// when not paused.
func New(interval time.Duration) *ticker {
	return &ticker{
		interval: interval,
	}
}

// Ticks returns a receive-only channel that delivers times at the ticker's
// prescribed interval. This method returns nil when the ticker is paused.
//
// NOTE: Part of the Ticker interface.
func (t *ticker) Ticks() <-chan time.Time {
	if t.ticker == nil {
		return nil
	}
	return t.ticker.C
}

// Resumes starts underlying time.Ticker and causes the ticker to begin
// delivering events.
//
// NOTE: Part of the Ticker interface.
func (t *ticker) Start() {
	if t.ticker != nil {
		return
	}
	t.ticker = time.NewTicker(t.interval)
}

// Pause suspends the underlying ticker, such that Ticks() stops signaling at
// regular intervals.
//
// NOTE: Part of the Ticker interface.
func (t *ticker) Stop() {
	if t.ticker != nil {
		t.ticker.Stop()
		t.ticker = nil
	}
}
