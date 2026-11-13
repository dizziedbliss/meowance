package portal

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the odd-term student/parent login page of MCE's Contineo portal.
const DefaultBaseURL = "https://mceparents.contineo.in/parents/index.php"

// Errors a caller can tell apart to show the right message.
var (
	// ErrBadLogin: the portal rejected the USN + date of birth.
	ErrBadLogin = errors.New("portal: wrong usn or date of birth")
	// ErrPortalDown: the portal didn't answer, or answered with a server error.
	ErrPortalDown = errors.New("portal: unreachable")
	// ErrPortalChanged: the portal answered but its markup isn't what we parse.
	ErrPortalChanged = errors.New("portal: page layout changed")
	// ErrForbidden: the portal (or its firewall) answered 403: it's blocking our IP.
	ErrForbidden = errors.New("portal: access forbidden")
)

const maxBody = 5 << 20 // 5 MB; the dashboard is ~60 KB

// A real mobile Chrome. Without browser-like headers the portal 403s us as a bot.
const userAgent = "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"

var (
	// Joomla's CSRF token: a hidden input whose name is 32 hex chars and value is 1.
	reToken = regexp.MustCompile(`<input[^>]+name="([0-9a-f]{32})"[^>]*value="1"`)
	// The login form's own "return" field (the first non-empty one inside #login-form).
	reLoginForm = regexp.MustCompile(`(?s)<form[^>]+id="login-form".*?</form>`)
	reReturn    = regexp.MustCompile(`name="return"\s+value="([^"]*)"`)
	// An error page's <title>, to tell a firewall block from a portal error in logs.
	reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

// Client logs into the portal and fetches one student's dashboard.
// It is safe for concurrent use: every Fetch gets its own cookie jar, so one
// student's session can never leak into another's.
type Client struct {
	BaseURL   string
	Transport http.RoundTripper
	Timeout   time.Duration // per request
	Retries   int           // extra attempts after a network failure or a 403 (never after a bad login)
	Backoff   time.Duration // wait between attempts
	// Logf, if set, gets one line per extra page (timetable, calendar): status, size,
	// what was parsed. Never credentials, never URLs (they carry the session).
	// It's called from parallel page fetches, so it must be safe for that
	// (log.Printf is).
	Logf func(format string, args ...any)

	proxy *proxySession // nil: direct connection
}

func (c *Client) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// NewClient returns a Client for the real portal. With OXYLABS_USERNAME and
// OXYLABS_PASSWORD set it goes through the residential proxy; without, direct.
func NewClient() *Client {
	c := &Client{BaseURL: DefaultBaseURL, Timeout: 25 * time.Second, Retries: 2, Backoff: 1500 * time.Millisecond}
	if user, pass := os.Getenv("OXYLABS_USERNAME"), os.Getenv("OXYLABS_PASSWORD"); user != "" && pass != "" {
		c.useProxy(newProxySession(user, pass, proxyAddr))
	}
	return c
}

func (c *Client) useProxy(p *proxySession) {
	c.proxy = p
	c.Transport = p.transport()
}

// Proxied reports whether requests go through the residential proxy.
func (c *Client) Proxied() bool { return c.proxy != nil }

// Extras are the pages fetched after the dashboard, only when asked for: the
// browser caches them (the timetable for a week, the calendar for days), so most
// logins need neither.
type Extras struct {
	Timetable, Calendar, Exams bool
	History                    []string // course codes whose class-by-class attendance is wanted
	CIE                        []string // course codes whose CIE page (class averages) is wanted
}

// Any reports whether any extra page is wanted.
func (e Extras) Any() bool {
	return e.Timetable || e.Calendar || e.Exams || len(e.History) > 0 || len(e.CIE) > 0
}

// maxParallel caps the extra pages fetched at once, so a fresh browser asking
// for everything doesn't hit the portal (and the proxy) with a burst.
const maxParallel = 6

// Snapshot is everything one login fetches.
type Snapshot struct {
	Dashboard *Dashboard
	// Asked is which extras were requested; the two fields below are only
	// meaningful for those.
	Asked Extras
	// Timetable is this week's classes, or nil if that page couldn't be fetched
	// (the dashboard still counts as a success; the timetable is a bonus).
	Timetable []Day
	// Calendar is the semester's calendar of events, or nil if it couldn't be fetched.
	Calendar *Calendar
	// History and CIE are keyed by course code: a nil value means that page
	// couldn't be fetched; a code not on the dashboard is left out.
	History map[string]*CourseHistory
	CIE     map[string]*CIEDetails
	// Exams is every past semester's results, or nil if it couldn't be fetched.
	Exams *ExamHistory
}

// setHeaders makes a request look like the browser the User-Agent claims to be.
// No Accept-Encoding: Go only decompresses gzip for us when it sets that itself.
func setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
}

// Fetch logs in with usn + dob and returns the parsed dashboard plus whichever
// extras were asked for. Credentials are used for this one exchange and never
// stored or logged.
func (c *Client) Fetch(ctx context.Context, usn string, dob time.Time, want Extras) (*Snapshot, error) {
	var err error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ErrPortalDown
			case <-time.After(c.Backoff):
			}
		}
		actx, session := ctx, ""
		if c.proxy != nil {
			session = c.proxy.current()
			actx = withSession(ctx, session) // one exit IP for this whole attempt
		}
		var s *Snapshot
		s, err = c.fetchOnce(actx, usn, dob, want)
		switch {
		case errors.Is(err, ErrForbidden):
			// Only a new exit IP can change a 403. Retrying may log in again, which
			// is harmless: a wrong birthday comes back as the login page, never a
			// 403, so it can't be counted twice.
			if c.proxy == nil {
				return nil, err
			}
			c.logf("portal: 403 on attempt %d/%d, rotating proxy session", attempt+1, c.Retries+1)
			c.proxy.rotate(session)
		case errors.Is(err, ErrPortalDown):
		default:
			return s, err // success, bad login, or changed markup: retrying won't help
		}
	}
	return nil, err
}

func (c *Client) fetchOnce(ctx context.Context, usn string, dob time.Time, want Extras) (*Snapshot, error) {
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Jar: jar, Transport: c.Transport, Timeout: c.Timeout}

	// 1. Login page: session cookie + CSRF token.
	page, err := c.get(ctx, hc, c.BaseURL, "")
	if err != nil {
		return nil, err
	}
	tok := reToken.FindStringSubmatch(page)
	if tok == nil {
		return nil, fmt.Errorf("%w: no login token", ErrPortalChanged)
	}
	ret := ""
	if f := reLoginForm.FindString(page); f != "" {
		if m := reReturn.FindStringSubmatch(f); m != nil {
			ret = m[1]
		}
	}

	// 2. Log in. The page's JS sends the DOB as YYYY-MM-DD in "passwd"; its
	// "encryption" is dead code, so we send exactly what a browser sends.
	form := url.Values{}
	form.Set("username", usn)
	form.Set("dd", dob.Format("02"))
	form.Set("mm", dob.Format("01"))
	form.Set("yyyy", dob.Format("2006"))
	form.Set("passwd", dob.Format("2006-01-02"))
	form.Set("remember", "No")
	form.Set("option", "com_user")
	form.Set("task", "login")
	form["return"] = []string{ret, ""}
	form.Set(tok[1], "1")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	setHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", c.BaseURL)
	resp, err := hc.Do(req) // follows the 303s to the dashboard, cookies included
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrPortalDown, netKind(err))
	}
	dashURL := resp.Request.URL.String() // final, signed dashboard URL: the Referer for later pages
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	// 3. Parse. A login page coming back means the portal said no.
	d, err := ParseDashboard(body)
	if errors.Is(err, ErrNoCourses) {
		if strings.Contains(body, `name="passwd"`) {
			return nil, ErrBadLogin
		}
		return nil, fmt.Errorf("%w: dashboard had no courses", ErrPortalChanged)
	}
	if err != nil {
		return nil, err
	}

	// 4. Asked-for extras, same session, like clicking through from the
	// dashboard; in parallel (the cookie jar is safe for that). Best effort:
	// any of them failing still returns the dashboard.
	snap := &Snapshot{Dashboard: d, Asked: want}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex // guards the snapshot's maps
		slot = make(chan struct{}, maxParallel)
	)
	fetchPage := func(name, path string, use func(string)) {
		wg.Go(func() {
			slot <- struct{}{}
			defer func() { <-slot }()
			if p, ok := c.extra(ctx, hc, name, path, dashURL); ok {
				use(p)
			}
		})
	}
	if want.Timetable {
		fetchPage("timetable", TimetablePath, func(p string) {
			snap.Timetable = ParseTimetable(p)
			c.logf("timetable: %d bytes, %d days", len(p), len(snap.Timetable))
		})
	}
	if want.Calendar {
		fetchPage("calendar", CalendarPath, func(p string) {
			snap.Calendar = ParseCalendar(p)
			c.logf("calendar: %d bytes, parsed %t", len(p), snap.Calendar != nil)
		})
	}
	if want.Exams {
		fetchPage("exam history", fmt.Sprintf(examHistoryPath, url.QueryEscape(usnOf(d, usn))), func(p string) {
			x, err := ParseExamHistory(p)
			if err == nil {
				snap.Exams = x
			}
			c.logf("exam history: %d bytes, parsed %t", len(p), err == nil)
		})
	}
	courses := map[string]Course{}
	for _, co := range d.Courses {
		courses[co.Code] = co
	}
	// Every asked course gets its nil ("couldn't fetch") entry before any
	// per-course fetch starts, so the goroutines only overwrite entries, under mu.
	var history, cie []Course
	if len(want.History) > 0 {
		snap.History = map[string]*CourseHistory{}
	}
	for _, code := range want.History {
		if co, ok := courses[code]; ok {
			snap.History[code] = nil
			history = append(history, co)
		}
	}
	if len(want.CIE) > 0 {
		snap.CIE = map[string]*CIEDetails{}
	}
	for _, code := range want.CIE {
		if co, ok := courses[code]; ok {
			snap.CIE[code] = nil
			cie = append(cie, co)
		}
	}
	for _, co := range history {
		fetchPage("attendance list", fmt.Sprintf(attendanceListPath, co.CourseID, co.SecID, co.SemID), func(p string) {
			h, err := ParseAttendanceList(p)
			mu.Lock()
			if err == nil {
				snap.History[co.Code] = h
			}
			mu.Unlock()
			c.logf("attendance list: %d bytes, parsed %t", len(p), err == nil)
		})
	}
	for _, co := range cie {
		code := co.Code
		fetchPage("cie details", fmt.Sprintf(cieDetailsPath, co.CourseID, co.SemID), func(p string) {
			x, err := ParseCIEDetails(p)
			mu.Lock()
			if err == nil {
				snap.CIE[code] = x
			}
			mu.Unlock()
			c.logf("cie details: %d bytes, parsed %t", len(p), err == nil)
		})
	}
	wg.Wait()
	return snap, nil
}

// usnOf is the USN for the exam-history link: the dashboard's, else the login's.
func usnOf(d *Dashboard, login string) string {
	if d.USN != "" {
		return d.USN
	}
	return login
}

// extra fetches one more page of the logged-in session and logs what came back.
func (c *Client) extra(ctx context.Context, hc *http.Client, name, path, referer string) (string, bool) {
	page, err := c.get(ctx, hc, c.BaseURL+path, referer)
	if err != nil {
		c.logf("%s: %v", name, err)
		return "", false
	}
	if what := pageKind(page); what != name {
		c.logf("%s: got the %s instead (%d bytes)", name, what, len(page))
	}
	return page, true
}

// pageKind guesses which portal page some HTML is, for debugging.
func pageKind(html string) string {
	switch {
	case strings.Contains(html, `name="passwd"`):
		return "login page"
	case strings.Contains(html, "cn-time-table"):
		return "timetable"
	case strings.Contains(html, "events-lists") || strings.Contains(html, "Start Date"):
		return "calendar"
	case strings.Contains(html, "cn-attend-list1"):
		return "attendance list"
	case strings.Contains(html, "res-table"):
		return "exam history"
	case strings.Contains(html, "cn-cie-grid"):
		return "cie details"
	case strings.Contains(html, "attendencelist"):
		return "dashboard"
	}
	return "unknown page"
}

func (c *Client) get(ctx context.Context, hc *http.Client, u, referer string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	setHeaders(req)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrPortalDown, netKind(err))
	}
	return readBody(resp)
}

// netKind describes a transport error without its URL (which carries the session).
func netKind(err error) string {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "network error"
}

// pageTitle is an error page's <title>, trimmed, for logs.
func pageTitle(body []byte) string {
	m := reTitle.FindSubmatch(body)
	if m == nil {
		return "(no title)"
	}
	t := html.UnescapeString(strings.TrimSpace(string(m[1])))
	if len(t) > 80 {
		t = t[:80]
	}
	return t
}

func readBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("%w: status %d", ErrPortalDown, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("%w: status %d, title %q", ErrForbidden, resp.StatusCode, pageTitle(body))
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d, title %q", ErrPortalChanged, resp.StatusCode, pageTitle(body))
	}
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrPortalDown, netKind(err))
	}
	return string(body), nil
}
