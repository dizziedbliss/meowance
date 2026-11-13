package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/meowance/pkg/portal"
)

type fakeFetcher struct {
	d   *portal.Snapshot
	err error
	got struct {
		usn  string
		dob  time.Time
		want portal.Extras
	}
}

// Fetch behaves like the real client: only the asked-for extras come back.
func (f *fakeFetcher) Fetch(_ context.Context, usn string, dob time.Time, want portal.Extras) (*portal.Snapshot, error) {
	f.got.usn, f.got.dob, f.got.want = usn, dob, want
	if f.d == nil {
		return nil, f.err
	}
	s := *f.d
	s.Asked = want
	if !want.Timetable {
		s.Timetable = nil
	}
	if !want.Calendar {
		s.Calendar = nil
	}
	if !want.Exams {
		s.Exams = nil
	}
	s.History, s.CIE = nil, nil
	for _, code := range want.History {
		if s.History == nil {
			s.History = map[string]*portal.CourseHistory{}
		}
		s.History[code] = f.d.History[code]
	}
	for _, code := range want.CIE {
		if s.CIE == nil {
			s.CIE = map[string]*portal.CIEDetails{}
		}
		s.CIE[code] = f.d.CIE[code]
	}
	return &s, f.err
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
	rec := post(Attendance(f), `{"usn":" 4mc00aa000 ","dob":"15082004","extras":["timetable","calendar"]}`)
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
	var days []portal.Day
	var cal *portal.Calendar
	json.Unmarshal(resp.Timetable, &days)
	json.Unmarshal(resp.Calendar, &cal)
	if len(days) != 4 {
		t.Errorf("timetable days = %d, want 4", len(days))
	}
	if cal == nil || cal.End != "2026-11-20" || len(cal.Events) != 25 {
		t.Errorf("calendar = %+v", cal)
	}
	// a course with no CIE yet must serialise as [] (the UI maps over it), not null
	if !strings.Contains(rec.Body.String(), `"code":"23AI703"`) || strings.Contains(rec.Body.String(), `"cie":null`) {
		t.Errorf("cie should never be null: %s", rec.Body.String())
	}
}

func TestAttendanceTimetableUnavailable(t *testing.T) {
	snap := fixture(t)
	snap.Timetable = nil
	rec := post(Attendance(&fakeFetcher{d: snap}), `{"usn":"4MC00AA000","dob":"15082004","extras":["timetable"]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"timetable":null`) {
		t.Fatalf("want 200 with timetable:null, got %d %s", rec.Code, rec.Body.String())
	}
}

// Extras not asked for are left out entirely ("not fetched", so the browser
// keeps its cached copy), never sent as null ("fetch failed").
func TestAttendanceOnlyAskedExtras(t *testing.T) {
	cases := []struct {
		body                 string
		want                 portal.Extras
		hasTimetable, hasCal bool
	}{
		{`{"usn":"4MC00AA000","dob":"15082004"}`, portal.Extras{}, false, false},
		{`{"usn":"4MC00AA000","dob":"15082004","extras":[]}`, portal.Extras{}, false, false},
		{`{"usn":"4MC00AA000","dob":"15082004","extras":["calendar"]}`, portal.Extras{Calendar: true}, false, true},
		{`{"usn":"4MC00AA000","dob":"15082004","extras":["timetable","calendar"]}`, portal.Extras{Timetable: true, Calendar: true}, true, true},
	}
	for _, c := range cases {
		f := &fakeFetcher{d: fixture(t)}
		rec := post(Attendance(f), c.body)
		body := rec.Body.String()
		if rec.Code != http.StatusOK || !reflect.DeepEqual(f.got.want, c.want) {
			t.Fatalf("%s: status %d, fetcher asked for %+v", c.body, rec.Code, f.got.want)
		}
		if strings.Contains(body, `"timetable":`) != c.hasTimetable || strings.Contains(body, `"calendar":`) != c.hasCal {
			t.Errorf("%s: response timetable=%t calendar=%t, want %t/%t", c.body,
				strings.Contains(body, `"timetable":`), strings.Contains(body, `"calendar":`), c.hasTimetable, c.hasCal)
		}
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
		{"unknown extra", `{"usn":"4MC00AA001","dob":"15082004","extras":["fees"]}`, nil, 400, CodeBadInput},
		{"bad course code", `{"usn":"4MC00AA001","dob":"15082004","extras":["history:../../x"]}`, nil, 400, CodeBadInput},
		{"history without code", `{"usn":"4MC00AA001","dob":"15082004","extras":["history:"]}`, nil, 400, CodeBadInput},
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

func (hangingFetcher) Fetch(ctx context.Context, _ string, _ time.Time, _ portal.Extras) (*portal.Snapshot, error) {
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

func TestAttendanceHistoryExtras(t *testing.T) {
	snap := fixture(t)
	read := func(name string) string {
		b, err := os.ReadFile("../portal/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	h, _ := portal.ParseAttendanceList(read("attendancelist.html"))
	cie, _ := portal.ParseCIEDetails(read("ciedetails.html"))
	snap.Exams, _ = portal.ParseExamHistory(read("examhistory.html"))
	snap.History = map[string]*portal.CourseHistory{"23AI701": h, "23AI702": nil} // 23AI702's page failed
	snap.CIE = map[string]*portal.CIEDetails{"23AI701": cie}
	f := &fakeFetcher{d: snap}
	rec := post(Attendance(f), `{"usn":"4MC00AA000","dob":"15082004","extras":["history:23AI701","history:23AI702","history:23AI701","cie:23AI701","exams"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if !reflect.DeepEqual(f.got.want, portal.Extras{History: []string{"23AI701", "23AI702"}, CIE: []string{"23AI701"}, Exams: true}) {
		t.Errorf("asked the portal for %+v (duplicates should collapse)", f.got.want)
	}
	var resp struct {
		History    map[string]*portal.CourseHistory `json:"history"`
		CIEDetails map[string]*portal.CIEDetails    `json:"cieDetails"`
		Exams      *portal.ExamHistory              `json:"exams"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if got := resp.History["23AI701"]; got == nil || got.StillToGo != 20 || len(got.Classes) != 25 {
		t.Errorf("history 23AI701 = %+v", got)
	}
	if v, ok := resp.History["23AI702"]; !ok || v != nil {
		t.Errorf("a failed page should be null, got %v (present %t)", v, ok)
	}
	if resp.CIEDetails["23AI701"] == nil || resp.Exams == nil || len(resp.Exams.Sems) != 6 {
		t.Errorf("cie = %+v, exams = %+v", resp.CIEDetails, resp.Exams)
	}
	if strings.Contains(rec.Body.String(), `"timetable"`) {
		t.Error("timetable sent without being asked")
	}
}

func TestAttendanceCapsCourseExtras(t *testing.T) {
	var extras []string
	for i := range maxCourses + 1 {
		extras = append(extras, fmt.Sprintf(`"history:23AI%03d"`, i))
	}
	body := `{"usn":"4MC00AA000","dob":"15082004","extras":[` + strings.Join(extras, ",") + `]}`
	if rec := post(Attendance(&fakeFetcher{d: fixture(t)}), body); rec.Code != http.StatusBadRequest {
		t.Fatalf("%d course extras: status %d, want 400", maxCourses+1, rec.Code)
	}
}
