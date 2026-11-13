package portal

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The three pages behind the dashboard's per-course and history links. Each is
// fetched only when the browser asks for it (see Extras), and cached there.

// Paths of the pages the dashboard links to. courseId/secId/semId come from the
// dashboard's own links (Course.CourseID etc.).
const (
	attendanceListPath = "?option=com_studentdashboard&controller=studentdashboard&task=attendencelist&courseId=%s&secId=%s&semId=%s"
	cieDetailsPath     = "?option=com_studentdashboard&controller=studentdashboard&task=ciedetails&courseId=%s&secId=&semId=%s"
	examHistoryPath    = "?option=com_history&task=getSemResult&usn=%s"
)

// ClassRecord is one class of one course: when, and whether you were there.
type ClassRecord struct {
	Date    string `json:"date"`  // YYYY-MM-DD
	Start   string `json:"start"` // 11:00
	End     string `json:"end"`
	Present bool   `json:"present"`
}

// CourseHistory is one course's class-by-class attendance. StillToGo is the
// portal's own count of classes still planned this semester.
type CourseHistory struct {
	Code      string        `json:"code"`
	Name      string        `json:"name"`
	Faculty   string        `json:"faculty"` // the lecturer's name; their photo is never read
	Present   int           `json:"present"`
	Absent    int           `json:"absent"`
	StillToGo int           `json:"stillToGo"`
	Classes   []ClassRecord `json:"classes"` // oldest first
}

var (
	reComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	reLecturer    = regexp.MustCompile(`(?s)class="md-card-head-text"[^>]*>\s*([^<]*?)\s*<br>\s*<span>\s*([0-9][0-9A-Za-z]+)\s*-\s*([^<]+?)\s*</span>`)
	reLegend      = regexp.MustCompile(`(Present|Absent|Still to go)\s*\[\s*(\d+)\s*\]`)
	reClassRow    = regexp.MustCompile(`(?s)<tr[^>]*>\s*<td>\s*\d+\s*</td>\s*<td>\s*(\d{2})-(\d{2})-(\d{4})\s*</td>\s*<td>\s*(\d{1,2}:\d{2})\s*TO\s*(\d{1,2}:\d{2})\s*</td>\s*<td>\s*(Present|Absent)\s*</td>`)
	reCIEHeading  = regexp.MustCompile(`<th colspan="\d+">\s*(?:<br/?>)?\s*[^<(]*\(\s*([0-9][0-9A-Za-z]+)\s*\)`)
	reCIEPoint    = regexp.MustCompile(`\{\s*"xaxis":\s*"([^"]+)",\s*"col1":\s*([0-9.]+),\s*"col2":\s*[0-9.]+,\s*"maxmarks":\s*([0-9.]+),\s*"optainmarks":\s*([0-9.]+)`)
	reSemTable    = regexp.MustCompile(`(?s)<table class="uk-table uk-table-striped res-table">(.*?)</table>`)
	reSemCaption  = regexp.MustCompile(`(?s)Semester:\s*(\d+).*?Cumulative Credits\s*:\s*([0-9.]+).*?SGPA:\s*([0-9.]+).*?CGPA\s*:\s*([0-9.]+)`)
	reResultRow   = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
	reCell        = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	reTag         = regexp.MustCompile(`<[^>]+>`)
	reCreditsCard = regexp.MustCompile(`(?s)<h3>\s*(Credits\s+Earned\s+So\s+Far|Credits\s+to\s+be\s+Earned|CGPA)\s*</h3>\s*<p>\s*([0-9.]+)\s*</p>`)
	reSpaces      = regexp.MustCompile(`\s+`)
)

// ParseAttendanceList reads one course's attendance page.
func ParseAttendanceList(html string) (*CourseHistory, error) {
	live := reComment.ReplaceAllString(html, "") // the page keeps an old, commented-out card
	m := reLecturer.FindStringSubmatch(live)
	if m == nil || !strings.Contains(live, "cn-attend-list1") {
		return nil, fmt.Errorf("%w: not an attendance list", ErrPortalChanged)
	}
	h := &CourseHistory{Faculty: m[1], Code: m[2], Name: m[3], Classes: []ClassRecord{}}
	for _, l := range reLegend.FindAllStringSubmatch(live, -1) {
		n := int(num(l[2]))
		switch l[1] {
		case "Present":
			h.Present = n
		case "Absent":
			h.Absent = n
		case "Still to go":
			h.StillToGo = n
		}
	}
	for _, r := range reClassRow.FindAllStringSubmatch(live, -1) {
		h.Classes = append(h.Classes, ClassRecord{
			Date: r[3] + "-" + r[2] + "-" + r[1], Start: pad(r[4]), End: pad(r[5]), Present: r[6] == "Present",
		})
	}
	sort.SliceStable(h.Classes, func(i, j int) bool {
		a, b := h.Classes[i], h.Classes[j]
		return a.Date+a.Start < b.Date+b.Start
	})
	return h, nil
}

// pad makes "9:30" sort before "11:00".
func pad(t string) string {
	if len(t) == 4 {
		return "0" + t
	}
	return t
}

// CIEStat is one internal test next to the class average.
type CIEStat struct {
	Name     string  `json:"name"` // CIE1, CIE2, Final IA
	Obtained float64 `json:"obtained"`
	ClassAvg float64 `json:"classAvg"`
	Max      float64 `json:"max"`
}

// CIEDetails is one course's CIE page: your marks and the class's.
type CIEDetails struct {
	Code  string    `json:"code"`
	Stats []CIEStat `json:"stats"`
}

// ParseCIEDetails reads the numbers behind the CIE page's chart.
func ParseCIEDetails(html string) (*CIEDetails, error) {
	code := reCIEHeading.FindStringSubmatch(html)
	points := reCIEPoint.FindAllStringSubmatch(html, -1)
	if code == nil || len(points) == 0 {
		return nil, fmt.Errorf("%w: not a cie details page", ErrPortalChanged)
	}
	d := &CIEDetails{Code: code[1]}
	for _, p := range points {
		d.Stats = append(d.Stats, CIEStat{Name: p[1], ClassAvg: num(p[2]), Max: num(p[3]), Obtained: num(p[4])})
	}
	return d, nil
}

// ExamCourse is one course in a past semester's results.
type ExamCourse struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	CIE     float64 `json:"cie"`
	SEE     float64 `json:"see"`
	Total   int     `json:"total"`
	Date    string  `json:"date"` // exam session as printed, e.g. Feb-2024; "" when the portal has none
	Credits float64 `json:"credits"`
	Earned  float64 `json:"earned"`
	Audit   bool    `json:"audit"` // NSS / yoga / PE: no credits
	GPA     float64 `json:"gpa"`
	Grade   string  `json:"grade"`
}

// SemResult is one semester's results. Credits is cumulative, as the portal prints it.
type SemResult struct {
	Sem     int          `json:"sem"`
	Credits float64      `json:"credits"`
	SGPA    float64      `json:"sgpa"`
	CGPA    float64      `json:"cgpa"`
	Courses []ExamCourse `json:"courses"`
}

// ExamHistory is every past semester plus the running totals.
type ExamHistory struct {
	CreditsEarned float64     `json:"creditsEarned"`
	CreditsToEarn float64     `json:"creditsToEarn"`
	CGPA          float64     `json:"cgpa"`
	Sems          []SemResult `json:"sems"`
}

// ParseExamHistory reads the exam history page.
func ParseExamHistory(html string) (*ExamHistory, error) {
	tables := reSemTable.FindAllStringSubmatch(html, -1)
	if len(tables) == 0 {
		return nil, fmt.Errorf("%w: not an exam history page", ErrPortalChanged)
	}
	x := &ExamHistory{Sems: []SemResult{}}
	for _, c := range reCreditsCard.FindAllStringSubmatch(html, -1) {
		switch reSpaces.ReplaceAllString(c[1], " ") {
		case "Credits Earned So Far":
			x.CreditsEarned = num(c[2])
		case "Credits to be Earned":
			x.CreditsToEarn = num(c[2])
		case "CGPA":
			x.CGPA = num(c[2])
		}
	}
	for _, t := range tables {
		cap := reSemCaption.FindStringSubmatch(t[1])
		if cap == nil {
			continue
		}
		s := SemResult{Sem: int(num(cap[1])), Credits: num(cap[2]), SGPA: num(cap[3]), CGPA: num(cap[4]), Courses: []ExamCourse{}}
		for _, r := range reResultRow.FindAllStringSubmatch(t[1], -1) {
			cells := reCell.FindAllStringSubmatch(r[1], -1)
			if len(cells) != 10 {
				continue // the header row
			}
			v := make([]string, len(cells))
			for i, c := range cells {
				v[i] = strings.TrimSpace(reTag.ReplaceAllString(reComment.ReplaceAllString(c[1], ""), ""))
			}
			date := v[5]
			if strings.HasSuffix(date, "-1970") { // the portal's "no date"
				date = ""
			}
			s.Courses = append(s.Courses, ExamCourse{
				Code: v[0], Name: v[1], CIE: num(v[2]), SEE: num(v[3]), Total: int(num(v[4])), Date: date,
				Credits: num(v[6]), Earned: num(v[7]), Audit: v[6] == "Audit", GPA: num(v[8]), Grade: v[9],
			})
		}
		x.Sems = append(x.Sems, s)
	}
	return x, nil
}
