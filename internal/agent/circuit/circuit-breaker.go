package circuit

// Reused Functionality

import (
	"log/slog"
	"sync"
	"time"
)

const numBuckets = 10

type window struct {
	buckets   [numBuckets]bucket
	bucketDur time.Duration
	current   int
	lastTime  time.Time
}

type bucket struct {
	successes int
	failures  int
}

type State int

const (
	StateClosed   State = 0
	StateOpen     State = 1
	StateHalfOpen State = 2
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

func newSlidingWindow(windowSize time.Duration, now time.Time) window {
	return window{
		bucketDur: windowSize / numBuckets,
		lastTime:  now,
	}
}

func (w *window) advance(now time.Time) {
	elapsed := now.Sub(w.lastTime)
	steps := int(elapsed / w.bucketDur)
	if steps <= 0 {
		return
	}
	if steps > numBuckets {
		steps = numBuckets
	}
	for i := 0; i < steps; i++ {
		w.current = (w.current + 1) % numBuckets
		w.buckets[w.current] = bucket{}
	}
	w.lastTime = now
}

func (w *window) recordSuccess(now time.Time) {
	w.advance(now)
	w.buckets[w.current].successes++
}

func (w *window) recordFailure(now time.Time) {
	w.advance(now)
	w.buckets[w.current].failures++
}

func (w *window) totals(now time.Time) (successes, failures int) {
	w.advance(now)
	for i := 0; i < numBuckets; i++ {
		successes += w.buckets[i].successes
		failures += w.buckets[i].failures
	}
	return
}

// Breaker implements a per-replica circuit breaker with three states:
//   - Closed: requests flow normally; failures tracked in a sliding window.
//   - Open: requests rejected immediately; transitions to HalfOpen after cooldown.
//   - HalfOpen: one probe request allowed; success closes, failure re-opens.
type Breaker struct {
	id         string // replica ID, used in log messages
	mu         sync.Mutex
	state      State
	window     window
	openedAt   time.Time
	cooldown   time.Duration
	threshold  float64
	minSamples int
	now        func() time.Time // injectable clock for testing
}

func newBreaker(id string, threshold float64, windowSize, cooldown time.Duration, minSamples int, now func() time.Time) *Breaker {
	return &Breaker{
		id:         id,
		state:      StateClosed,
		window:     newSlidingWindow(windowSize, now()),
		cooldown:   cooldown,
		threshold:  threshold,
		minSamples: minSamples,
		now:        now,
	}
}

// Allow reports whether a request may be sent through this breaker.
// In the HalfOpen state, only the first caller after the cooldown gets through.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if b.now().Sub(b.openedAt) >= b.cooldown {
			b.state = StateHalfOpen
			return true // one probe allowed
		}
		return false
	case StateHalfOpen:
		return false // probe already in flight
	default:
		return true
	}
}

// RecordSuccess records a successful request. In HalfOpen, this closes the
// circuit and resets the sliding window.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	b.window.recordSuccess(now)
	if b.state == StateHalfOpen {
		b.state = StateClosed
		b.window = newSlidingWindow(b.window.bucketDur*numBuckets, now)
		slog.Info("circuit breaker closed", "replica", b.id)
	}
}

// RecordFailure records a failed request and potentially trips the breaker.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	b.window.recordFailure(now)

	switch b.state {
	case StateClosed:
		successes, failures := b.window.totals(now)
		total := successes + failures
		if total >= b.minSamples {
			errorRate := float64(failures) / float64(total)
			if errorRate >= b.threshold {
				b.state = StateOpen
				b.openedAt = now
				slog.Warn("circuit breaker opened",
					"replica", b.id,
					"error_rate", errorRate,
					"threshold", b.threshold,
					"total", total,
				)
			}
		}
	case StateHalfOpen:
		b.state = StateOpen
		b.openedAt = now
		slog.Warn("circuit breaker re-opened (probe failed)", "replica", b.id)
	}
}

// State returns the current breaker state, accounting for cooldown expiry.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateOpen && b.now().Sub(b.openedAt) >= b.cooldown {
		b.state = StateHalfOpen
	}
	return b.state
}

// ErrorRate returns the current error rate across the sliding window.
func (b *Breaker) ErrorRate() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	successes, failures := b.window.totals(b.now())
	total := successes + failures
	if total == 0 {
		return 0
	}
	return float64(failures) / float64(total)
}
