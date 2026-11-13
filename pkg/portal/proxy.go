package portal

import (
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

// url is the http.Transport Proxy func: read on every new connection, so a
// rotation applies to the very next request.
func (p *proxySession) url(*http.Request) (*url.URL, error) {
	p.mu.Lock()
	id := p.id
	p.mu.Unlock()
	name := fmt.Sprintf("customer-%s-cc-%s-city-%s-sessid-%s", p.user, proxyCountry, proxyCity, id)
	return &url.URL{Scheme: "http", User: url.UserPassword(name, p.pass), Host: p.addr}, nil
}

// rotate switches to a new session ID (a new exit IP).
func (p *proxySession) rotate() {
	p.mu.Lock()
	p.id = newSessionID()
	tr := p.tr
	p.mu.Unlock()
	if tr != nil {
		tr.CloseIdleConnections()
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
