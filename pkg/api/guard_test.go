package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dizziedbliss/meowance/pkg/portal"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func newClock() *clock { return &clock{t: time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)} }

// postFrom sends a login from a given client IP.
func postFrom(h http.HandlerFunc, ip, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/attendance", strings.NewReader(body))
	req.Header.Set("X-Real-IP", ip)
	h(rec, req)
	return rec
}

func code(rec *httptest.ResponseRecorder) string {
	var m map[string]string
	json.Unmarshal(rec.Body.Bytes(), &m)
	return m["error"]
}

const victim = `{"usn":"4MC00AA001","dob":"15082004"}`

func TestLimiterBlocksGuessingFromOneIP(t *testing.T) {
	c := newClock()
	f := &fakeFetcher{err: portal.ErrBadLogin}
	h := attendance(f, newLimiter(c.now))
	for i := range triesPerIP {
		if rec := postFrom(h, "1.1.1.1", victim); rec.Code != http.StatusUnauthorized {
			t.Fatalf("try %d: %d, want 401", i+1, rec.Code)
		}
	}
	f.got.usn = ""
	rec := postFrom(h, "1.1.1.1", victim)
	if rec.Code != http.StatusTooManyRequests || code(rec) != CodeTooManyTries || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d %q, want 429 too_many_tries with Retry-After", rec.Code, code(rec))
	}
	if f.got.usn != "" {
		t.Fatal("a blocked attempt must not reach the portal")
	}
	c.add(triesWindow + time.Second)
	if rec := postFrom(h, "1.1.1.1", victim); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after the window: %d, want 401", rec.Code)
	}
}

// One stranger guessing your birthday must not lock YOU out.
func TestLimiterNoTargetedLockout(t *testing.T) {
	h := attendance(&fakeFetcher{err: portal.ErrBadLogin}, newLimiter(newClock().now))
	for range triesPerIP + 3 {
		postFrom(h, "6.6.6.6", victim) // attacker burns their own budget
	}
	if rec := postFrom(h, "9.9.9.9", victim); rec.Code != http.StatusUnauthorized {
		t.Fatalf("the real student on another network got %d, want to reach the portal (401)", rec.Code)
	}
}

// Guessing spread across many IPs still hits the per-USN wall.
func TestLimiterCapsDistributedGuessing(t *testing.T) {
	f := &fakeFetcher{err: portal.ErrBadLogin}
	h := attendance(f, newLimiter(newClock().now))
	reached := 0
	for i := range triesPerUSN + 10 {
		ip := "10.0.0." + string(rune('a'+i%26)) + string(rune('a'+i/26))
		if rec := postFrom(h, ip, victim); rec.Code == http.StatusUnauthorized {
			reached++
		}
	}
	if reached != triesPerUSN {
		t.Fatalf("%d guesses reached the portal, want exactly %d", reached, triesPerUSN)
	}
}

// slowWrong answers "wrong birthday" after a pause, to make requests overlap.
type slowWrong struct{ calls atomic.Int32 }

func (s *slowWrong) Fetch(ctx context.Context, _ string, _ time.Time, _ portal.Extras) (*portal.Snapshot, error) {
	s.calls.Add(1)
	time.Sleep(30 * time.Millisecond)
	return nil, portal.ErrBadLogin
}

// 50 guesses fired at once must not all slip past the check.
func TestLimiterParallelGuesses(t *testing.T) {
	f := &slowWrong{}
	h := attendance(f, newLimiter(newClock().now))
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() { postFrom(h, "1.1.1.1", victim) })
	}
	wg.Wait()
	if n := f.calls.Load(); n != triesPerIP {
		t.Fatalf("%d parallel guesses reached the portal, want %d", n, triesPerIP)
	}
}

func TestLimiterSuccessAndOutagesDontCount(t *testing.T) {
	c := newClock()
	lim := newLimiter(c.now)
	wrong := attendance(&fakeFetcher{err: portal.ErrBadLogin}, lim)
	down := attendance(&fakeFetcher{err: portal.ErrPortalDown}, lim)
	right := attendance(&fakeFetcher{d: fixture(t)}, lim)

	for range triesPerIP + 5 {
		if rec := postFrom(down, "1.1.1.1", victim); rec.Code != http.StatusBadGateway {
			t.Fatalf("portal down: %d, want 502 (outages must not use up tries)", rec.Code)
		}
	}
	blocked := attendance(&fakeFetcher{err: portal.ErrForbidden}, lim)
	for range triesPerIP + 5 {
		if rec := postFrom(blocked, "1.1.1.1", victim); rec.Code != http.StatusBadGateway {
			t.Fatalf("portal blocking us: %d, want 502 (our IP problem must not use up tries)", rec.Code)
		}
	}
	for range triesPerIP - 1 {
		postFrom(wrong, "1.1.1.1", victim)
	}
	if rec := postFrom(right, "1.1.1.1", victim); rec.Code != http.StatusOK {
		t.Fatalf("correct login: %d", rec.Code)
	}
	for i := range triesPerIP { // a full fresh allowance after a correct login
		if rec := postFrom(wrong, "1.1.1.1", victim); rec.Code != http.StatusUnauthorized {
			t.Fatalf("after success, try %d: %d", i+1, rec.Code)
		}
	}
}

// Flooding the table with junk keys must never flush a live count (blocked or
// partial); when it's full of live counts, new keys are refused, not tracked-free.
func TestGuardFloodNeverFlushesCounts(t *testing.T) {
	c := newClock()
	g := newGuard(3, time.Hour, c.now)
	g.cap = 50
	g.begin("victim") // a partial count: 1 of 3 used
	for i := 0; len(g.entries) < g.cap; i++ {
		g.begin("junk" + strconv.Itoa(i))
	}
	if _, ok, wait := g.begin("newcomer"); ok || wait != fullWait {
		t.Fatalf("full table: ok=%v wait=%v, want refused with %v", ok, wait, fullWait)
	}
	if n := len(g.entries["victim"]); n != 1 {
		t.Fatalf("victim's count = %d after the flood, want 1 (it was flushed)", n)
	}
	// a full table doesn't rescan on every request: the next sweep waits sweepEvery
	swept := g.swept
	g.begin("another")
	if g.swept != swept {
		t.Fatal("swept again within sweepEvery")
	}
	// once the junk expires, room frees up and nothing was lost early
	c.add(time.Hour + time.Second)
	if _, ok, _ := g.begin("newcomer"); !ok {
		t.Fatal("after expiry the table should have room again")
	}
}
