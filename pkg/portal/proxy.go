package portal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// The portal blocks datacenter IPs (Vercel's included), so production goes out
// through a residential proxy. Each session ID pins one exit IP ("sticky");
// rotating the ID gets a fresh IP when the portal starts answering 403.
const (
	proxyAddr    = "pr.oxylabs.io:7777"
	proxyCountry = "in"
	proxyCity    = "bengaluru"
)

// proxySession is a rotatable sticky session on the residential proxy.
type proxySession struct {
	user, pass, addr string

	mu sync.Mutex
	id string
	tr *http.Transport // closed idle connections on rotate, so no tunnel keeps the old IP
}

func newProxySession(user, pass, addr string) *proxySession {
	return &proxySession{user: user, pass: pass, addr: addr, id: newSessionID()}
}

type sessionKey struct{}

// withSession pins every request made with ctx to one session ID, so a
// rotation by another student's login can't move this one to a new IP midway
// (the portal's cookie and CSRF token were issued to the old one).
func withSession(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionKey{}, id)
}

// current is the session ID new logins should use.
func (p *proxySession) current() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.id
}

// url is the http.Transport Proxy func, called for every request. The session
// ID is part of the proxy URL, so a different ID gets its own pooled
// connections (and its own CONNECT tunnel) and a different exit IP.
func (p *proxySession) url(r *http.Request) (*url.URL, error) {
	id, _ := r.Context().Value(sessionKey{}).(string)
	if id == "" {
		id = p.current()
	}
	name := fmt.Sprintf("customer-%s-cc-%s-city-%s-sessid-%s", p.user, proxyCountry, proxyCity, id)
	return &url.URL{Scheme: "http", User: url.UserPassword(name, p.pass), Host: p.addr}, nil
}

// rotate retires a session ID that got blocked. Only the first of several
// logins that hit the same block rotates; the rest find it already replaced.
func (p *proxySession) rotate(blocked string) {
	p.mu.Lock()
	if p.id != blocked {
		p.mu.Unlock()
		return
	}
	p.id = newSessionID()
	tr := p.tr
	p.mu.Unlock()
	if tr != nil {
		tr.CloseIdleConnections() // drop the blocked IP's idle tunnels now, not in 30s
	}
}

// transport is an http.Transport that dials through this session.
func (p *proxySession) transport() *http.Transport {
	tr := &http.Transport{
		Proxy:                 p.url,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	p.mu.Lock()
	p.tr = tr
	p.mu.Unlock()
	return tr
}

func newSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano()) // still unique; crypto/rand never fails on Linux
	}
	return hex.EncodeToString(b)
}
