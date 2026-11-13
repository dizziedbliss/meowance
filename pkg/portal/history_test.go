package portal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseAttendanceList(t *testing.T) {
	h, err := ParseAttendanceList(readFixture(t, "attendancelist.html"))
	if err != nil {
		t.Fatal(err)
	}
	if h.Code != "23AI701" || h.Name != "Cloud Computing" || h.Faculty != "FACULTY" {
		t.Errorf("course = %q %q by %q", h.Code, h.Name, h.Faculty)
	}
	if h.Present != 18 || h.Absent != 6 || h.StillToGo != 21 {
		t.Errorf("counts = %d present, %d absent, %d to go; want 18/6/21", h.Present, h.Absent, h.StillToGo)
	}
	if len(h.Classes) != 24 {
		t.Fatalf("classes = %d, want 24 (18 present + 6 absent)", len(h.Classes))
	}
	// Oldest first, by date then start time; dates as YYYY-MM-DD.
	first, last := h.Classes[0], h.Classes[len(h.Classes)-1]
	if first != (ClassRecord{Date: "2026-08-11", Start: "11:00", End: "12:00", Present: true}) {
		t.Errorf("first = %+v", first)
	}
	if last.Date != "2026-09-30" || last.Start != "14:00" || last.End != "16:00" {
		t.Errorf("last = %+v", last)
	}
	for i := 1; i < len(h.Classes); i++ {
		a, b := h.Classes[i-1], h.Classes[i]
		if a.Date+a.Start > b.Date+b.Start {
			t.Fatalf("not sorted at %d: %+v then %+v", i, a, b)
		}
	}
	var absent []string
	for _, c := range h.Classes {
		if !c.Present {
			absent = append(absent, c.Date)
		}
	}
	if strings.Join(absent, ",") != "2026-08-12,2026-08-26,2026-09-02,2026-09-15,2026-09-22,2026-09-30" {
		t.Errorf("absent dates = %v", absent)
	}
}

func TestParseCIEDetails(t *testing.T) {
	d, err := ParseCIEDetails(readFixture(t, "ciedetails.html"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Code != "23AI701" {
		t.Errorf("code = %q", d.Code)
	}
	want := []CIEStat{
		{Name: "CIE1", Obtained: 6, ClassAvg: 6, Max: 10},
		{Name: "CIE2", Obtained: 7, ClassAvg: 7, Max: 10},
		{Name: "Final IA", Obtained: 13, ClassAvg: 14, Max: 50},
	}
	if len(d.Stats) != len(want) {
		t.Fatalf("stats = %+v", d.Stats)
	}
	for i := range want {
		if d.Stats[i] != want[i] {
			t.Errorf("stat %d = %+v, want %+v", i, d.Stats[i], want[i])
		}
	}
}

func TestParseExamHistory(t *testing.T) {
	x, err := ParseExamHistory(readFixture(t, "examhistory.html"))
	if err != nil {
		t.Fatal(err)
	}
	if x.CreditsEarned != 124 || x.CreditsToEarn != 36 || x.CGPA != 7.90 {
		t.Errorf("summary = %v earned, %v to go, cgpa %v", x.CreditsEarned, x.CreditsToEarn, x.CGPA)
	}
	if len(x.Sems) != 6 {
		t.Fatalf("sems = %d, want 6", len(x.Sems))
	}
	s1, s6 := x.Sems[0], x.Sems[5]
	if s1.Sem != 1 || s1.Credits != 20 || s1.SGPA != 7.70 || s1.CGPA != 7.70 {
		t.Errorf("sem 1 = %+v", s1)
	}
	if s6.Sem != 6 || s6.Credits != 124 || s6.CGPA != 7.90 {
		t.Errorf("sem 6 = %+v", s6)
	}
	n := 0
	for _, s := range x.Sems {
		n += len(s.Courses)
	}
	if n != 55 {
		t.Errorf("courses = %d, want 55", n)
	}
	c := s1.Courses[0]
	if c != (ExamCourse{Code: "23ESC145", Name: "Introduction to C Programming", CIE: 37, SEE: 31, Total: 68, Date: "Mar-2024", Credits: 3, Earned: 3, GPA: 7, Grade: "B+"}) {
		t.Errorf("first course = %+v", c)
	}
	var audit *ExamCourse
	for _, s := range x.Sems {
		for i := range s.Courses {
			if s.Courses[i].Code == "23NYP1" {
				audit = &s.Courses[i]
			}
		}
	}
	if audit == nil || !audit.Audit || audit.Credits != 0 || audit.Date != "" {
		t.Errorf("audit course = %+v (want Audit, no credits, junk Jan-1970 date dropped)", audit)
	}
}

func TestHistoryParsersRejectOtherPages(t *testing.T) {
	login := readFixture(t, "login.html")
	if _, err := ParseAttendanceList(login); !errors.Is(err, ErrPortalChanged) {
		t.Errorf("attendance list on login page: %v", err)
	}
	if _, err := ParseCIEDetails(login); !errors.Is(err, ErrPortalChanged) {
		t.Errorf("cie details on login page: %v", err)
	}
	if _, err := ParseExamHistory(login); !errors.Is(err, ErrPortalChanged) {
		t.Errorf("exam history on login page: %v", err)
	}
}
