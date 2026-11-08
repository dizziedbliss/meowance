package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/dizziedbliss/meowance/internal/portal"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestGuardBlocksBirthdayGuessing(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)}
	f := &fakeFetcher{err: portal.ErrBadLogin}
	h := attendance(f, newGuard(maxWrongTries, triesWindow, c.now))
	body := `{"usn":"4MC00AA001","dob":"15082004"}`

	for i := range maxWrongTries {
		if rec := post(h, body); rec.Code != http.StatusUnauthorized {
			t.Fatalf("try %d: status %d, want 401", i+1, rec.Code)
		}
	}
	// the next guess is refused without asking the portal
	f.got.usn = ""
	rec := post(h, body)
	var got map[string]string
	json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusTooManyRequests || got["error"] != CodeTooManyTries || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d %v (Retry-After %q), want 429 too_many_tries", rec.Code, got, rec.Header().Get("Retry-After"))
	}
	if f.got.usn != "" {
		t.Fatal("a blocked USN must not reach the portal")
	}

	// other USNs are unaffected
	if rec := post(h, `{"usn":"4MC00AA002","dob":"15082004"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("other usn: status %d, want 401", rec.Code)
	}

	// after the window it's allowed again
	c.t = c.t.Add(triesWindow + time.Second)
	if rec := post(h, body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after window: status %d, want 401 (portal asked again)", rec.Code)
	}
}

func TestGuardResetsOnSuccess(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)}
	g := newGuard(maxWrongTries, triesWindow, c.now)
	bad := &fakeFetcher{err: portal.ErrBadLogin}
	for range maxWrongTries - 1 {
		post(attendance(bad, g), `{"usn":"4MC00AA001","dob":"15082004"}`)
	}
	if rec := post(attendance(&fakeFetcher{d: fixture(t)}, g), `{"usn":"4MC00AA001","dob":"15082004"}`); rec.Code != http.StatusOK {
		t.Fatalf("correct login: status %d", rec.Code)
	}
	if blocked, _ := g.blocked("4MC00AA001"); blocked {
		t.Fatal("a successful login should clear the count")
	}
	for i := range maxWrongTries - 1 { // a fresh allowance, not one left over
		if rec := post(attendance(bad, g), `{"usn":"4MC00AA001","dob":"15082004"}`); rec.Code != http.StatusUnauthorized {
			t.Fatalf("after reset, try %d: %d", i+1, rec.Code)
		}
	}
}
