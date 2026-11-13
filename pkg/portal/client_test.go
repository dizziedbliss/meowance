package portal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testUSN   = "4MC00AA000"
	testToken = "5cf5d64a03d1ecc7f4d09459f292ea8e"
)

var testDOB = time.Date(2004, time.August, 15, 0, 0, 0, 0, time.UTC)

// fakePortal mimics the real flow: login page sets a session cookie, the POST
// checks the token + credentials and 303s, the dashboard needs the session.
func fakePortal(t *testing.T, loginPage string) (*httptest.Server, *int32) {
	t.Helper()
	dash, err := os.ReadFile(filepath.Join("testdata", "dashboard.html"))
	if err != nil {
		t.Fatal(err)
	}
	tt, err := os.ReadFile(filepath.Join("testdata", "timetable.html"))
	if err != nil {
		t.Fatal(err)
	}
	cal, err := os.ReadFile(filepath.Join("testdata", "calendar.html"))
	if err != nil {
		t.Fatal(err)
	}
	extraPages := map[string][]byte{}
	for task, file := range map[string]string{"attendencelist": "attendancelist.html", "ciedetails": "ciedetails.html", "getSemResult": "examhistory.html"} {
		b, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		extraPages[task] = b
	}
	var posts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := r.Cookie("sess")
		switch r.Method {
		case http.MethodGet:
			if c, _ := r.Cookie("in"); c != nil && sess != nil {
				switch r.URL.Query().Get("task") {
				case "timetable":
					if timetableDown.Load() {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
					w.Write(tt)
				case "calenderdisplay":
					w.Write(cal)
				case "attendencelist", "ciedetails", "getSemResult":
					w.Write(extraPages[r.URL.Query().Get("task")])
				default:
					w.Write(dash)
				}
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "sess", Value: "s1", Path: "/"})
			w.Write([]byte(loginPage))
		case http.MethodPost:
			atomic.AddInt32(&posts, 1)
			r.ParseForm()
			ok := sess != nil &&
				r.PostForm.Get(testToken) == "1" &&
				r.PostForm.Get("username") == testUSN &&
				r.PostForm.Get("passwd") == "2004-08-15" &&
				r.PostForm.Get("dd") == "15" && r.PostForm.Get("mm") == "08" && r.PostForm.Get("yyyy") == "2004" &&
				r.PostForm.Get("option") == "com_user" && r.PostForm.Get("task") == "login" &&
				r.PostForm["return"][0] == "LOGINRET"
			if ok {
				http.SetCookie(w, &http.Cookie{Name: "in", Value: "1", Path: "/"})
				http.Redirect(w, r, "/parents/index.php?option=com_studentdashboard&task=dashboard", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/parents/index.php", http.StatusSeeOther)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &posts
}

func loginFixture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "login.html"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func newTestClient(base string) *Client {
	return &Client{BaseURL: base + "/parents/index.php", Timeout: 5 * time.Second, Retries: 1, Backoff: time.Millisecond}
}

// timetableDown makes the fake portal fail only the timetable page.
var timetableDown atomic.Bool

func TestFetchSuccess(t *testing.T) {
	srv, _ := fakePortal(t, loginFixture(t))
	s, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, all)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	d := s.Dashboard
	if len(d.Courses) != 5 || d.Courses[0].Code != "23AI701" || d.Courses[0].Attended != 18 {
		t.Fatalf("unexpected dashboard: %+v", d.Courses)
	}
	if len(s.Timetable) != 4 || s.Timetable[0].Weekday != "Monday" {
		t.Fatalf("timetable = %+v, want Mon–Thu", s.Timetable)
	}
	if s.Calendar == nil || s.Calendar.End != "2026-11-20" {
		t.Fatalf("calendar = %+v, want sem ending 2026-11-20", s.Calendar)
	}
}

func TestFetchLogsNeverLeakSession(t *testing.T) {
	srv, _ := fakePortal(t, loginFixture(t))
	var mu sync.Mutex
	var lines []string
	c := newTestClient(srv.URL)
	c.Logf = func(f string, a ...any) { // called from parallel page fetches
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, fmt.Sprintf(f, a...))
	}
	if _, err := c.Fetch(context.Background(), testUSN, testDOB, all); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("want one log line per extra page, got %q", lines)
	}
	for _, l := range lines {
		if strings.Contains(l, testUSN) || strings.Contains(l, "2004") || strings.Contains(l, "http") {
			t.Errorf("log line leaks credentials or a URL: %q", l)
		}
	}
}

func TestFetchTimetableIsBestEffort(t *testing.T) {
	srv, _ := fakePortal(t, loginFixture(t))
	timetableDown.Store(true)
	defer timetableDown.Store(false)
	s, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, all)
	if err != nil {
		t.Fatalf("a broken timetable page must not fail the login: %v", err)
	}
	if s.Timetable != nil || len(s.Dashboard.Courses) != 5 {
		t.Fatalf("want dashboard with nil timetable, got timetable=%v", s.Timetable)
	}
}

func TestFetchBadLogin(t *testing.T) {
	srv, posts := fakePortal(t, loginFixture(t))
	_, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB.AddDate(0, 0, 1), all)
	if !errors.Is(err, ErrBadLogin) {
		t.Fatalf("err = %v, want ErrBadLogin", err)
	}
	if *posts != 1 {
		t.Fatalf("posted %d times; a bad login must not be retried", *posts)
	}
}

func TestFetchNoToken(t *testing.T) {
	srv, _ := fakePortal(t, "<html><form id=\"login-form\"></form></html>")
	_, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, all)
	if !errors.Is(err, ErrPortalChanged) {
		t.Fatalf("err = %v, want ErrPortalChanged", err)
	}
}

func TestFetchPortalDownRetries(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	_, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, all)
	if !errors.Is(err, ErrPortalDown) {
		t.Fatalf("err = %v, want ErrPortalDown", err)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2 (one try + one retry)", hits)
	}
}

func TestFetchSessionsDoNotLeak(t *testing.T) {
	srv, _ := fakePortal(t, loginFixture(t))
	c := newTestClient(srv.URL)
	if _, err := c.Fetch(context.Background(), testUSN, testDOB, all); err != nil {
		t.Fatal(err)
	}
	// Same client, wrong DOB: must not ride the previous session's cookies.
	if _, err := c.Fetch(context.Background(), testUSN, testDOB.AddDate(0, 0, 1), all); !errors.Is(err, ErrBadLogin) {
		t.Fatalf("second fetch err = %v, want ErrBadLogin (session leaked?)", err)
	}
}

func TestFetchPostsLoginOnce(t *testing.T) {
	srv, posts := fakePortal(t, loginFixture(t))
	if _, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, all); err != nil {
		t.Fatal(err)
	}
	if *posts != 1 {
		t.Fatalf("posted the login %d times, want 1", *posts)
	}
}

func TestReadBodyStatuses(t *testing.T) {
	cases := []struct {
		status int
		want   error
		title  string
	}{
		{200, nil, ""},
		{403, ErrForbidden, `"Access Denied"`},
		{404, ErrPortalChanged, `"Access Denied"`},
		{503, ErrPortalDown, ""},
	}
	for _, c := range cases {
		resp := &http.Response{StatusCode: c.status, Body: io.NopCloser(strings.NewReader("<html><title> Access Denied </title>"))}
		_, err := readBody(resp)
		if !errors.Is(err, c.want) || (c.want == nil) != (err == nil) {
			t.Errorf("status %d: err = %v, want %v", c.status, err, c.want)
		}
		if c.title != "" && !strings.Contains(err.Error(), c.title) {
			t.Errorf("status %d: error %q should carry the page title %s", c.status, err, c.title)
		}
	}
}

func TestPageTitle(t *testing.T) {
	if got := pageTitle([]byte("<HTML><Title lang=en>\n Rock &amp; Roll </Title>")); got != "Rock & Roll" {
		t.Fatalf("got %q", got)
	}
	if got := pageTitle([]byte("no title here")); got != "(no title)" {
		t.Fatalf("got %q", got)
	}
}

// A portal that hangs must not run past the caller's deadline, retries included.
func TestFetchStopsAtDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	c.Retries, c.Backoff = 5, 10*time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Fetch(ctx, testUSN, testDOB, all)
	if !errors.Is(err, ErrPortalDown) || time.Since(start) > time.Second {
		t.Fatalf("err = %v after %v, want ErrPortalDown right at the deadline", err, time.Since(start))
	}
}

var all = Extras{Timetable: true, Calendar: true}

// pageHits counts GETs per portal task, to see which pages a login fetched.
func pageHits(t *testing.T, srv *httptest.Server) *sync.Map {
	t.Helper()
	hits := &sync.Map{}
	inner := srv.Config.Handler
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			n, _ := hits.LoadOrStore(r.URL.Query().Get("task"), new(atomic.Int32))
			n.(*atomic.Int32).Add(1)
		}
		inner.ServeHTTP(w, r)
	})
	return hits
}

func hitCount(m *sync.Map, task string) int32 {
	n, ok := m.Load(task)
	if !ok {
		return 0
	}
	return n.(*atomic.Int32).Load()
}

func TestFetchOnlyAskedExtras(t *testing.T) {
	cases := []struct {
		want           Extras
		timetable, cal int32
	}{
		{Extras{}, 0, 0},
		{Extras{Timetable: true}, 1, 0},
		{Extras{Calendar: true}, 0, 1},
		{all, 1, 1},
	}
	for _, c := range cases {
		srv, _ := fakePortal(t, loginFixture(t))
		hits := pageHits(t, srv)
		s, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, c.want)
		if err != nil {
			t.Fatalf("%+v: %v", c.want, err)
		}
		if got := [2]int32{hitCount(hits, "timetable"), hitCount(hits, "calenderdisplay")}; got != [2]int32{c.timetable, c.cal} {
			t.Errorf("%+v: fetched timetable/calendar %v times, want %d/%d", c.want, got, c.timetable, c.cal)
		}
		if !reflect.DeepEqual(s.Asked, c.want) || (s.Timetable != nil) != c.want.Timetable || (s.Calendar != nil) != c.want.Calendar {
			t.Errorf("%+v: snapshot asked=%+v timetable=%t calendar=%t", c.want, s.Asked, s.Timetable != nil, s.Calendar != nil)
		}
	}
}

func TestFetchHistoryPages(t *testing.T) {
	srv, _ := fakePortal(t, loginFixture(t))
	hits := pageHits(t, srv)
	want := Extras{History: []string{"23AI701", "NOT-A-COURSE"}, CIE: []string{"23AI701"}, Exams: true}
	s, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, want)
	if err != nil {
		t.Fatal(err)
	}
	if got := [3]int32{hitCount(hits, "attendencelist"), hitCount(hits, "ciedetails"), hitCount(hits, "getSemResult")}; got != [3]int32{1, 1, 1} {
		t.Errorf("fetched attendance list/cie/exams %v times, want 1 each (unknown codes skipped)", got)
	}
	if h := s.History["23AI701"]; h == nil || len(h.Classes) != 24 {
		t.Errorf("history = %+v", s.History)
	}
	if _, asked := s.History["NOT-A-COURSE"]; asked {
		t.Error("a code not on the dashboard should be left out, not fetched")
	}
	if d := s.CIE["23AI701"]; d == nil || len(d.Stats) != 3 {
		t.Errorf("cie = %+v", s.CIE)
	}
	if s.Exams == nil || len(s.Exams.Sems) != 6 {
		t.Errorf("exams = %+v", s.Exams)
	}
	if s.Timetable != nil || s.Calendar != nil {
		t.Error("timetable/calendar fetched without being asked")
	}
}

func TestFetchAllHistory(t *testing.T) {
	srv, _ := fakePortal(t, loginFixture(t))
	hits := pageHits(t, srv)
	s, err := newTestClient(srv.URL).Fetch(context.Background(), testUSN, testDOB, Extras{AllHistory: true})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(s.Dashboard.Courses); hitCount(hits, "attendencelist") != int32(n) || len(s.History) != n {
		t.Errorf("fetched %d attendance lists, history has %d entries; want one per course (%d)", hitCount(hits, "attendencelist"), len(s.History), n)
	}
	if len(s.Asked.History) != len(s.Dashboard.Courses) || s.Asked.AllHistory {
		t.Errorf("Asked = %+v; want the expanded course list", s.Asked)
	}
}
