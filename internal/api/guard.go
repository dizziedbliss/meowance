package api

import (
	"sync"
	"time"
)

// guard slows down birthday guessing: after max wrong logins for one USN within
// window, that USN is refused without contacting the portal until the window passes.
//
// It lives in memory, so on Vercel each warm instance keeps its own count. That's a
// speed bump, not a wall; the Vercel firewall's per-IP rate limit is the other half.
type guard struct {
	mu     sync.Mutex
	fails  map[string][]time.Time
	max    int
	window time.Duration
	now    func() time.Time
}

// maxTracked bounds memory: past this many USNs the map is reset (fails open, briefly).
const maxTracked = 10_000

func newGuard(max int, window time.Duration, now func() time.Time) *guard {
	return &guard{fails: map[string][]time.Time{}, max: max, window: window, now: now}
}

// recent drops failures older than the window. Callers hold g.mu.
func (g *guard) recent(usn string) []time.Time {
	cutoff := g.now().Add(-g.window)
	kept := g.fails[usn][:0]
	for _, t := range g.fails[usn] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(g.fails, usn)
		return nil
	}
	g.fails[usn] = kept
	return kept
}

// blocked reports whether usn has used up its wrong guesses, and for how long it stays blocked.
func (g *guard) blocked(usn string) (bool, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.recent(usn)
	if len(r) < g.max {
		return false, 0
	}
	return true, r[0].Add(g.window).Sub(g.now())
}

func (g *guard) fail(usn string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.fails) >= maxTracked {
		g.fails = map[string][]time.Time{}
	}
	g.fails[usn] = append(g.recent(usn), g.now())
}

func (g *guard) reset(usn string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.fails, usn)
}
