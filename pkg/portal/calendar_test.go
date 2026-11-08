package portal

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseCalendar(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "calendar.html"))
	if err != nil {
		t.Fatal(err)
	}
	cal := ParseCalendar(string(b))
	if cal == nil {
		t.Fatal("ParseCalendar returned nil")
	}
	if cal.Start != "2026-08-03" || cal.End != "2026-11-20" {
		t.Errorf("sem = %s..%s, want 2026-08-03..2026-11-20", cal.Start, cal.End)
	}
	if len(cal.Holidays) != 23 || len(cal.ExamDays) != 9 {
		t.Errorf("holidays=%d examDays=%d, want 23 and 9", len(cal.Holidays), len(cal.ExamDays))
	}
	for _, d := range []string{"2026-08-15", "2026-11-10"} {
		if !slices.Contains(cal.Holidays, d) {
			t.Errorf("holidays missing %s", d)
		}
	}

	byTitle := map[string]CalendarEvent{}
	for _, e := range cal.Events {
		byTitle[e.Title] = e
	}
	want := []CalendarEvent{
		{From: "2026-08-03", To: "2026-08-03", Title: "commencement of classes", Kind: ""},
		{From: "2026-08-04", To: "2026-08-06", Title: "Registration of Regular courses", Kind: ""},
		{From: "2026-08-15", To: "2026-08-15", Title: "Independence Day", Kind: KindHoliday},
		{From: "2026-08-10", To: "2026-08-10", Title: "Registration with penal fee of Rs.500/- ( only offline)", Kind: ""},
		{From: "2026-11-12", To: "2026-11-14", Title: "Minor Exam 3", Kind: KindExam},
		{From: "2026-11-10", To: "2026-11-10", Title: "Balipadyami", Kind: KindHoliday},
		{From: "2026-11-20", To: "2026-11-20", Title: "LAST WORKING DAY", Kind: KindLastDay},
	}
	for _, w := range want {
		if got, ok := byTitle[w.Title]; !ok || got != w {
			t.Errorf("event %q = %+v, want %+v", w.Title, got, w)
		}
	}
	if len(cal.Events) != 25 { // every events-lists line on the page: 8 aug, 3 sep, 5 oct, 9 nov
		t.Errorf("got %d events, want 25", len(cal.Events))
	}
}

func TestParseCalendarRejectsOtherPages(t *testing.T) {
	if ParseCalendar("<html>Login to Your Account</html>") != nil {
		t.Error("want nil for a page without semester dates")
	}
}

func TestEventCrossesYear(t *testing.T) {
	ev, ok := parseEvent("Dec 30th to Jan 2nd - Winter break", 2026, 12)
	if !ok || ev.From != "2026-12-30" || ev.To != "2027-01-02" {
		t.Fatalf("got %+v ok=%v", ev, ok)
	}
}
