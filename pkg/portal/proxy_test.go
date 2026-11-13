package portal

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeProxy is a plain HTTP forward proxy that 403s every request from a
// blocked session ID, the way the portal's firewall treats a burnt exit IP.
type fakeProxy struct {
	mu       sync.Mutex
	seen     []string // session IDs, in order
	blocked  map[string]bool
	blockAll bool
}

func (p *fakeProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, pass := proxyCreds(r)
	if pass != "secret" || !strings.HasPrefix(user, "customer-u-cc-in-city-bengaluru-sessid-") {
		w.WriteHeader(http.StatusProxyAuthRequired)
		return
	}
	id := user[strings.LastIndex(user, "-")+1:]
	p.mu.Lock()
	p.seen = append(p.seen, id)
	if p.blocked == nil {
		p.blocked = map[string]bool{id: true} // the first session is burnt
	}
	blocked := p.blockAll || p.blocked[id]
	p.mu.Unlock()
	if blocked {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "<html><title>Access Denied</title></html>")
		return
	}
	out := r.Clone(r.Context())
	out.RequestURI = ""
	out.Header.Del("Proxy-Authorization")
	resp, err := http.DefaultTransport.RoundTrip(out)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func proxyCreds(r *http.Request) (string, string) {
	raw, ok := strings.CutPrefix(r.Header.Get("Proxy-Authorization"), "Basic ")
	if !ok {
		return "", ""
	}
	b, _ := base64.StdEncoding.DecodeString(raw)
	user, pass, _ := strings.Cut(string(b), ":")
	return user, pass
}

func (p *fakeProxy) sessions() map[string]bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]bool{}
	for _, id := range p.seen {
		out[id] = true
	}
	return out
}

func proxiedClient(t *testing.T, portal string) (*Client, *fakeProxy) {
	t.Helper()
	fp := &fakeProxy{}
	ps := httptest.NewServer(fp)
	t.Cleanup(ps.Close)
	c := newTestClient(portal)
	c.useProxy(newProxySession("u", "secret", strings.TrimPrefix(ps.URL, "http://")))
	return c, fp
}

func TestFetchForbiddenRotatesProxySession(t *testing.T) {
	srv, posts := fakePortal(t, loginFixture(t))
	c, fp := proxiedClient(t, srv.URL)
	snap, err := c.Fetch(context.Background(), testUSN, testDOB, all)
	if err != nil {
		t.Fatalf("Fetch: %v (want success on the rotated session)", err)
	}
	if snap.Dashboard == nil {
		t.Fatal("no dashboard")
	}
	if n := len(fp.sessions()); n != 2 {
		t.Fatalf("used %d proxy sessions, want 2 (burnt one + one rotation)", n)
	}
	if *posts != 1 {
		t.Fatalf("posted the login %d times, want exactly 1", *posts)
	}
}

func TestFetchForbiddenGivesUpAfterRetries(t *testing.T) {
	srv, posts := fakePortal(t, loginFixture(t))
	c, fp := proxiedClient(t, srv.URL)
	fp.blockAll = true
	_, err := c.Fetch(context.Background(), testUSN, testDOB, all)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if n := len(fp.sessions()); n != c.Retries+1 {
		t.Fatalf("used %d sessions, want %d (a fresh one per attempt)", n, c.Retries+1)
	}
	if *posts != 0 {
		t.Fatalf("login reached the portal %d times through a blocked proxy", *posts)
	}
}

func TestFetchForbiddenWithoutProxyFailsFast(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, all)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1: same IP, same 403, no point retrying", hits)
	}
}

func TestProxySessionRotate(t *testing.T) {
	p := newProxySession("u", "secret", "proxy:1")
	first := p.current()
	p.rotate(first)
	second := p.current()
	if second == first {
		t.Fatal("rotate kept the blocked session ID")
	}
	p.rotate(first) // a second login reporting the same block: already handled
	if p.current() != second {
		t.Fatal("a stale rotate replaced the fresh session")
	}
	u, _ := p.url(httptest.NewRequest(http.MethodGet, "/", nil))
	if pw, _ := u.User.Password(); pw != "secret" || u.Host != "proxy:1" || !strings.HasSuffix(u.User.Username(), second) {
		t.Fatalf("proxy URL lost its auth, host or session: %s", u.Redacted())
	}
}

// A login in flight keeps its exit IP even when another login rotates.
func TestProxySessionPinnedPerLogin(t *testing.T) {
	p := newProxySession("u", "secret", "proxy:1")
	pinned := p.current()
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(withSession(context.Background(), pinned))
	p.rotate(pinned)
	u, _ := p.url(req)
	if !strings.HasSuffix(u.User.Username(), pinned) {
		t.Fatalf("in-flight request moved to %s after another login rotated", u.User.Username())
	}
}
