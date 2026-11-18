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

func TestCalendarYearFor(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "calendar.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ branch, sem, want string }{
		{"B.E-AA", "07", "2"}, {"B.E-AA", "7", "2"}, {"B.E-AA", "05", "1"}, {"B.E-AA", "03", "3"},
		{"B.E-AA", "01", "4"}, {"B.E-AA", "04", ""}, {"B.E-AA", "x", ""},
		{"MCA", "03", "11"}, {"M.Tech-CS", "3", "12"}, {"MCA", "07", ""},
	} {
		if got := CalendarYearFor(string(b), c.branch, c.sem); got != c.want {
			t.Errorf("%s sem %q: id %q, want %q", c.branch, c.sem, got, c.want)
		}
	}
	// an old year still listed for the same sem: the newer (higher id) one wins
	two := "<a href='x?yearId=3'>COE FOR THE UG 3rd SEMESTER 2025-2026</a><a href='x?yearId=9'>COE FOR THE UG 3rd SEMESTER 2026-2027</a>"
	if got := CalendarYearFor(two, "B.E-AA", "3"); got != "9" {
		t.Errorf("two sem-3 COEs: id %q, want 9", got)
	}
}

func TestEventsYear(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "dashboard.html"))
	if err != nil {
		t.Fatal(err)
	}
	if got := EventsYear(string(b)); got != "2" {
		t.Errorf("EventsYear = %q, want 2 (the fixture's Events link)", got)
	}
	if got := EventsYear("<html>no menu</html>"); got != "" {
		t.Errorf("EventsYear without a link = %q, want empty", got)
	}
}
