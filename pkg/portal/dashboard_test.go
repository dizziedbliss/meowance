package portal

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) *Dashboard {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "dashboard.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	d, err := ParseDashboard(string(b))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return d
}

func TestParseHeader(t *testing.T) {
	d := loadFixture(t)
	if d.USN != "4MC00AA000" {
		t.Errorf("USN = %q, want 4MC00AA000 (scrubbed)", d.USN)
	}
	if d.Name != "STUDENT" {
		t.Errorf("Name = %q, want STUDENT (scrubbed)", d.Name)
	}
	if d.Branch != "B.E-CA" {
		t.Errorf("Branch = %q, want B.E-CA", d.Branch)
	}
	if d.Sem != "07" {
		t.Errorf("Sem = %q, want 07", d.Sem)
	}
	if d.Section != "A" {
		t.Errorf("Section = %q, want A", d.Section)
	}
	if d.LastUpdated != "08/11/2026" {
		t.Errorf("LastUpdated = %q, want 08/11/2026", d.LastUpdated)
	}
}

func TestParseCourses(t *testing.T) {
	d := loadFixture(t)
	want := []Course{
		{Code: "23AI701", Name: "Cloud Computing", LTPS: "2-0-2-0", Credits: 3, Type: "Theory", Nature: "Professional Studies Core", CourseID: "5669", SecID: "610", SemID: "226", Attended: 20, Conducted: 25},
		{Code: "23AI702", Name: "Prompt Engineering & AI Automation", LTPS: "2-0-2-0", Credits: 3, Type: "Theory", Nature: "Professional Studies Core", CourseID: "5670", SecID: "610", SemID: "226", Attended: 22, Conducted: 27},
		{Code: "23AI703", Name: "Full Stack Technologies", LTPS: "3-0-2-0", Credits: 4, Type: "Theory", Nature: "Professional Studies Core", CourseID: "5671", SecID: "610", SemID: "226", Attended: 25, Conducted: 30},
		{Code: "23AI704D", Name: "AI for Predictive Analytics and IoT", LTPS: "3-0-0-0", Credits: 3, Type: "Theory", Nature: "Professional Studies Electives", CourseID: "5672", SecID: "610", SemID: "226", Attended: 10, Conducted: 14},
		{Code: "23OEEC71", Name: "Sensors and Actuators", LTPS: "3-0-0-0", Credits: 3, Type: "Theory", Nature: "Other Electives", CourseID: "5616", SecID: "115", SemID: "38", Attended: 5, Conducted: 5},
	}
	if len(d.Courses) != len(want) {
		t.Fatalf("got %d courses, want %d", len(d.Courses), len(want))
	}
	for i, w := range want {
		got := d.Courses[i]
		if got.CIEMax != 50 || got.SEEMax != 50 {
			t.Errorf("course[%d] CIE/SEE max = %v/%v, want 50/50", i, got.CIEMax, got.SEEMax)
		}
		got.CIE, got.CIETotal, got.CIEMax, got.SEEMax = nil, 0, 0, 0 // covered by TestParseCIE
		if !reflect.DeepEqual(got, w) {
			t.Errorf("course[%d]\n got %+v\nwant %+v", i, got, w)
		}
	}
}

func TestParseCIE(t *testing.T) {
	d := loadFixture(t)
	cloud := d.Courses[0]
	want := []CIEPart{
		{Name: "SE1", Max: 10, Absent: true, Entered: "10th Sep 26"},
		{Name: "SE2", Max: 10, Obtained: 9, Entered: "13th Oct 26"},
	}
	if !reflect.DeepEqual(cloud.CIE, want) || cloud.CIETotal != 9 {
		t.Fatalf("23AI701 CIE = %+v total %v, want %+v total 9", cloud.CIE, cloud.CIETotal, want)
	}
	if full := d.Courses[2]; len(full.CIE) != 0 {
		t.Errorf("23AI703 should have no CIE parts yet, got %+v", full.CIE)
	}
}

func TestParseCIEPending(t *testing.T) {
	html := `<td class="top_back">23X1</td> CIE Details</td></tr><tr><td>SE3</td>
<td style="x">Marks Entered on </td></tr><tr><td><table><tr>
<td class="content_head">Max Marks</td><td class="content">20</td>
<td class="content_head">Obtained Marks</td><td class="content">-</td></tr></table>`
	got := detailsByCode(html)["23X1"].cie
	if len(got) != 1 || !got[0].Pending || got[0].Max != 20 {
		t.Fatalf("pending part = %+v", got)
	}
}

func TestParseTimetable(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "timetable.html"))
	if err != nil {
		t.Fatal(err)
	}
	days := ParseTimetable(string(b))
	if len(days) != 4 {
		t.Fatalf("got %d days, want 4 (Mon–Thu)", len(days))
	}
	if days[0].Weekday != "Monday" || days[0].Date != "02-11-2026" || len(days[0].Classes) != 4 || len(days[1].Classes) != 6 {
		t.Fatalf("days[0..1] = %+v / %d classes", days[0], len(days[1].Classes))
	}
	first := Class{Start: "09:30", End: "10:30", Code: "23AI704D", Name: "AI FOR PREDICTIVE ANALYTICS AND IOT", Room: "CLASS ROOM", Batch: "BATCH 1"}
	if days[0].Classes[0] != first {
		t.Errorf("first class = %+v, want %+v", days[0].Classes[0], first)
	}
	lab := days[0].Classes[3]
	if lab.Start != "14:00" || lab.End != "16:00" || lab.Room != "LABORATARY" || lab.Batch != "" {
		t.Errorf("lab slot = %+v", lab)
	}
	for _, d := range days {
		for _, c := range d.Classes {
			if strings.Contains(strings.ToUpper(c.Name+c.Room+c.Batch), "FACULTY") {
				t.Errorf("faculty column leaked into %+v", c)
			}
		}
	}
	if got := ParseTimetable("<html>no classes</html>"); len(got) != 0 {
		t.Errorf("empty page gave %d days", len(got))
	}
}

func TestParseRejectsNonDashboard(t *testing.T) {
	if _, err := ParseDashboard("<html><body>Login to Your Account</body></html>"); err != ErrNoCourses {
		t.Errorf("err = %v, want ErrNoCourses", err)
	}
}
