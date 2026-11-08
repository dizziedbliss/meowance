package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// guard counts login attempts per key inside a sliding window. An attempt is
// reserved BEFORE the portal is asked (so parallel guesses can't all slip past a
// check-then-count gap), kept if the birthday was wrong, and released otherwise.
//
// It lives in memory, so on Vercel each warm instance keeps its own counts: a
// speed bump, not a wall. The firewall's per-IP rate limit is the other half.
type guard struct {
	mu      sync.Mutex
	entries map[string][]attempt
	max     int
	window  time.Duration
	now     func() time.Time
	seq     uint64
	cap     int       // most keys tracked at once
	swept   time.Time // last full expiry sweep (sweeps are O(n), so they're throttled)
}

// sweepEvery limits full expiry sweeps when the table is full; between sweeps a
// full table refuses new keys immediately instead of rescanning under the lock.
const sweepEvery = 10 * time.Second

type attempt struct {
	at time.Time
	id uint64
}

// ticket is one reserved attempt; the zero ticket means "not tracked".
type ticket struct {
	key string
	id  uint64
}

// maxTracked bounds memory (~100k short keys is a few MB). Only expired keys are
// ever evicted; a count is never forgotten before its window ends. If the table is
// genuinely full of live counts, new keys are refused (fail closed) until room frees up.
const maxTracked = 100_000

// fullWait is the Retry-After for "table full": short, since entries expire continuously.
const fullWait = time.Minute

func newGuard(max int, window time.Duration, now func() time.Time) *guard {
	return &guard{entries: map[string][]attempt{}, max: max, window: window, now: now, cap: maxTracked}
}

// recent drops attempts older than the window. Callers hold g.mu.
func (g *guard) recent(key string) []attempt {
	cutoff := g.now().Add(-g.window)
	kept := g.entries[key][:0]
	for _, a := range g.entries[key] {
		if a.at.After(cutoff) {
			kept = append(kept, a)
		}
	}
	if len(kept) == 0 {
		delete(g.entries, key)
		return nil
	}
	g.entries[key] = kept
	return kept
}

// begin reserves an attempt for key. If key has no attempts left it returns
// ok=false and how long until the oldest one expires.
func (g *guard) begin(key string) (t ticket, ok bool, wait time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.recent(key)
	if len(r) >= g.max {
		return ticket{}, false, r[0].at.Add(g.window).Sub(g.now())
	}
	if r == nil && len(g.entries) >= g.cap && !g.evict() {
		return ticket{}, false, fullWait // full of live counts: refuse rather than forget any
	}
	g.seq++
	g.entries[key] = append(r, attempt{at: g.now(), id: g.seq})
	return ticket{key: key, id: g.seq}, true, 0
}

// evict frees room by dropping only keys whose attempts have all expired, so an
// attacker can't flush someone's count by flooding junk keys. Callers hold g.mu.
func (g *guard) evict() bool {
	if now := g.now(); now.Sub(g.swept) >= sweepEvery {
		g.swept = now
		for k := range g.entries {
			g.recent(k) // deletes the key when nothing is left in the window
		}
	}
	return len(g.entries) < g.cap
}

// release gives back an attempt that wasn't a wrong birthday (portal down, etc.).
func (g *guard) release(t ticket) {
	if t.key == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	v := g.entries[t.key]
	for i, a := range v {
		if a.id == t.id {
			g.entries[t.key] = append(v[:i], v[i+1:]...)
			break
		}
	}
	if len(g.entries[t.key]) == 0 {
		delete(g.entries, t.key)
	}
}

// reset clears a key after a correct login.
func (g *guard) reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}

// limiter combines two guards: a tight one per (USN, client IP), so a stranger
// can't lock you out from their network, and a looser one per USN, so guessing
// spread across many IPs still hits a wall.
type limiter struct {
	pair *guard // key: USN|IP
	usn  *guard // key: USN
}

// Attempt budgets per window.
const (
	triesPerIP  = 5
	triesPerUSN = 20
	triesWindow = 15 * time.Minute
)

func newLimiter(now func() time.Time) *limiter {
	return &limiter{pair: newGuard(triesPerIP, triesWindow, now), usn: newGuard(triesPerUSN, triesWindow, now)}
}

// pass is a reserved attempt on both guards.
type pass struct{ pair, usn ticket }

func (l *limiter) begin(usn, ip string) (pass, bool, time.Duration) {
	pt, ok, wait := l.pair.begin(usn + "|" + ip)
	if !ok {
		return pass{}, false, wait
	}
	ut, ok, wait := l.usn.begin(usn)
	if !ok {
		l.pair.release(pt)
		return pass{}, false, wait
	}
	return pass{pair: pt, usn: ut}, true, 0
}

// done handles every outcome except a wrong birthday (which keeps both
// reservations counted): a correct login clears the slate for that
// USN; anything else (portal down…) just gives the attempt back.
func (l *limiter) done(p pass, usn, ip string, success bool) {
	if success {
		l.pair.reset(usn + "|" + ip)
		l.usn.reset(usn)
		return
	}
	l.pair.release(p.pair)
	l.usn.release(p.usn)
}

// clientIP is the caller's address. On Vercel, X-Real-IP is set by the platform
// (a client can't override it); locally it falls back to the socket address.
func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
