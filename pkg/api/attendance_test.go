package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/meowance/pkg/portal"
)

type fakeFetcher struct {
	d   *portal.Snapshot
	err error
	got struct {
		usn string
		dob time.Time
	}
}

func (f *fakeFetcher) Fetch(_ context.Context, usn string, dob time.Time) (*portal.Snapshot, error) {
	f.got.usn, f.got.dob = usn, dob
	return f.d, f.err
}

func fixture(t *testing.T) *portal.Snapshot {
	t.Helper()
	b, err := os.ReadFile("../portal/testdata/dashboard.html")
	if err != nil {
		t.Fatal(err)
	}
	d, err := portal.ParseDashboard(string(b))
	if err != nil {
		t.Fatal(err)
	}
	tt, err := os.ReadFile("../portal/testdata/timetable.html")
	if err != nil {
		t.Fatal(err)
	}
	cal, err := os.ReadFile("../portal/testdata/calendar.html")
	if err != nil {
		t.Fatal(err)
	}
	return &portal.Snapshot{Dashboard: d, Timetable: portal.ParseTimetable(string(tt)), Calendar: portal.ParseCalendar(string(cal))}
}

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/api/attendance", strings.NewReader(body)))
	return rec
}

func TestAttendanceOK(t *testing.T) {
	f := &fakeFetcher{d: fixture(t)}
	rec := post(Attendance(f), `{"usn":" 4mc00aa000 ","dob":"15082004"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if f.got.usn != "4MC00AA000" || f.got.dob.Format("2006-01-02") != "2004-08-15" {
		t.Fatalf("fetcher got usn=%q dob=%v", f.got.usn, f.got.dob)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Student.Dept != "AA" || resp.Student.USN != "4MC00AA000" {
		t.Errorf("student = %+v", resp.Student)
	}
	want := map[string]Course{
		"23AI701":  {Percent: 80, Need75: 0, Need85: 9, Skip85: 0},
		"23AI704D": {Percent: 71.4, Need75: 2, Need85: 13, Skip85: 0},
		"23OEEC71": {Percent: 100, Need75: 0, Need85: 0, Skip85: 0},
	}
	for _, c := range resp.Courses {
		w, ok := want[c.Code]
		if !ok {
			continue
		}
		if c.Percent != w.Percent || c.Need75 != w.Need75 || c.Need85 != w.Need85 || c.Skip85 != w.Skip85 {
			t.Errorf("%s = %+v, want %+v", c.Code, c, w)
		}
	}
	if cloud := resp.Courses[0]; len(cloud.CIE) != 2 || cloud.CIETotal != 9 || cloud.CIEMax != 50 || !cloud.CIE[0].Absent {
		t.Errorf("23AI701 CIE = %+v total %v/%v", cloud.CIE, cloud.CIETotal, cloud.CIEMax)
	}
	if len(resp.Timetable) != 4 {
		t.Errorf("timetable days = %d, want 4", len(resp.Timetable))
	}
	if resp.Calendar == nil || resp.Calendar.End != "2026-11-20" || len(resp.Calendar.Events) != 25 {
		t.Errorf("calendar = %+v", resp.Calendar)
	}
	// a course with no CIE yet must serialise as [] (the UI maps over it), not null
	if !strings.Contains(rec.Body.String(), `"code":"23AI703"`) || strings.Contains(rec.Body.String(), `"cie":null`) {
		t.Errorf("cie should never be null: %s", rec.Body.String())
	}
}

func TestAttendanceTimetableUnavailable(t *testing.T) {
	snap := fixture(t)
	snap.Timetable = nil
	rec := post(Attendance(&fakeFetcher{d: snap}), `{"usn":"4MC00AA000","dob":"15082004"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"timetable":null`) {
		t.Fatalf("want 200 with timetable:null, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAttendanceErrors(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		err    error
		status int
		code   string
	}{
		{"not mce", `{"usn":"1RV22CS001","dob":"15082004"}`, nil, 400, CodeBadInput},
		{"bad date", `{"usn":"4MC00AA001","dob":"31022005"}`, nil, 400, CodeBadInput},
		{"future date", `{"usn":"4MC00AA001","dob":"01012999"}`, nil, 400, CodeBadInput},
		{"junk body", `nope`, nil, 400, CodeBadInput},
		{"wrong creds", `{"usn":"4MC00AA001","dob":"15082004"}`, portal.ErrBadLogin, 401, CodeBadLogin},
		{"portal down", `{"usn":"4MC00AA001","dob":"15082004"}`, portal.ErrPortalDown, 502, CodePortalDown},
		{"portal changed", `{"usn":"4MC00AA001","dob":"15082004"}`, portal.ErrPortalChanged, 502, CodePortalChanged},
		{"portal blocks us", `{"usn":"4MC00AA001","dob":"15082004"}`, portal.ErrForbidden, 502, CodePortalBlocked},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := post(Attendance(&fakeFetcher{err: c.err}), c.body)
			var got map[string]string
			json.Unmarshal(rec.Body.Bytes(), &got)
			if rec.Code != c.status || got["error"] != c.code {
				t.Fatalf("got %d %v, want %d %s", rec.Code, got, c.status, c.code)
			}
		})
	}
}

func TestAttendanceRejectsGET(t *testing.T) {
	rec := httptest.NewRecorder()
	Attendance(&fakeFetcher{})(rec, httptest.NewRequest(http.MethodGet, "/api/attendance", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d, want 405", rec.Code)
	}
}

// hangingFetcher is a portal that never answers: it waits for the deadline.
type hangingFetcher struct{}

func (hangingFetcher) Fetch(ctx context.Context, _ string, _ time.Time) (*portal.Snapshot, error) {
	<-ctx.Done()
	return nil, portal.ErrPortalDown
}

func TestAttendanceGivesUpWithinBudget(t *testing.T) {
	old := fetchBudget
	fetchBudget = 50 * time.Millisecond
	t.Cleanup(func() { fetchBudget = old })
	start := time.Now()
	rec := post(Attendance(hangingFetcher{}), `{"usn":"4MC00AA001","dob":"15082004"}`)
	if rec.Code != http.StatusBadGateway || time.Since(start) > time.Second {
		t.Fatalf("got %d after %v, want a 502 portal_down at the budget", rec.Code, time.Since(start))
	}
}
