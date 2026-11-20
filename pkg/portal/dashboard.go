// Package portal parses MCE's Contineo student-dashboard HTML into structured
// data. The dashboard is a single authenticated GET and already contains every
// course's attended/conducted counts and CIE marks (in per-course detail
// blocks), so no per-course drill-down is needed. Parsing is stdlib-only
// (regexp) — the markup is fixed, server-rendered, and we avoid an HTML dep.
package portal

import (
	"errors"
	stdhtml "html"
	"regexp"
	"strconv"
	"strings"
)

// CIEPart is one internal assessment (SE1, SE2, …).
type CIEPart struct {
	Name     string  `json:"name"`
	Max      float64 `json:"max"`
	Obtained float64 `json:"obtained"`
	Absent   bool    `json:"absent"`  // portal shows "A"
	Pending  bool    `json:"pending"` // no marks entered yet
	Entered  string  `json:"entered"` // as printed, e.g. "13th Oct 26"
}

// Course is one registered course with its current attendance and CIE marks.
type Course struct {
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	LTPS      string    `json:"ltps"`    // "2-0-2-0" (Lecture-Tutorial-Practical-Self study)
	Credits   float64   `json:"credits"` // 3.0, 4.0 ...
	Type      string    `json:"type"`    // "Theory"
	Nature    string    `json:"nature"`  // "Professional Studies Core"
	CIEMax    float64   `json:"cieMax"`  // usually 50
	SEEMax    float64   `json:"seeMax"`  // usually 50
	CourseID  string    `json:"courseId"`
	SecID     string    `json:"secId"`
	SemID     string    `json:"semId"`
	Attended  int       `json:"attended"`
	Conducted int       `json:"conducted"`
	CIE       []CIEPart `json:"cie"`
	CIETotal  float64   `json:"cieTotal"`
}

// Dashboard is the parsed student view.
type Dashboard struct {
	USN         string   `json:"usn"`
	Name        string   `json:"name"`
	Branch      string   `json:"branch"`      // "B.E-CA"
	Sem         string   `json:"sem"`         // "07"
	Section     string   `json:"section"`     // "A"
	LastUpdated string   `json:"lastUpdated"` // "08/11/2026" — freshness, surfaced to the user
	Courses     []Course `json:"courses"`
}

var (
	reUSN     = regexp.MustCompile(`name="usn"\s+value="([^"]+)"`)
	reName    = regexp.MustCompile(`<h3>([^<]+)</h3>\s*<img class="uk-preserve-width`)
	reBranch  = regexp.MustCompile(`<p>\s*([^,<]+?)\s*,\s*SEM\s*([0-9]+)\s*,\s*SEC\s*([A-Za-z0-9]+)\s*</p>`)
	reUpdated = regexp.MustCompile(`Last Updated On\s*:\s*([0-9/]+)`)
	// The course table, read row by row and cell by cell so one odd row (a
	// blank max, an audit course, no attendance link) can't swallow the next
	// row or vanish: code, name, LTPS, credits, type, nature, CIE max, SEE max.
	reTableRow    = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	reTD          = regexp.MustCompile(`(?s)<td([^>]*)>(.*?)</td>`)
	reCode        = regexp.MustCompile(`^[0-9]{2}[0-9A-Za-z]+$`)
	reListLink    = regexp.MustCompile(`attendencelist&courseId=([0-9]+)&secId=([0-9]*)&semId=([0-9]+)`)
	reDetailCode  = regexp.MustCompile(`top_back[^>]*>\s*([0-9][0-9A-Za-z]+)\s*</td>`)
	rePresentPair = regexp.MustCompile(`<span[^>]*>([0-9]+)</span>\s*/\s*<span[^>]*>([0-9]+)</span>`)
	reCIEPart     = regexp.MustCompile(`(?s)<td>\s*([^<]{1,20}?)\s*</td>\s*<td[^>]*>\s*Marks Entered on\s*([^<]*?)\s*</td>` +
		`.*?Max Marks</td>\s*<td[^>]*>\s*([0-9.]+)\s*</td>\s*<td[^>]*>\s*Obtained Marks</td>\s*<td[^>]*>\s*([^<]*?)\s*</td>`)
	reSEEName  = regexp.MustCompile(`(?i)^see\s*[0-9]*$`)
	reCIETotal = regexp.MustCompile(`(?s)Total Marks</td>\s*<td[^>]*>\s*([0-9.]+)\s*</td>`)
)

// ErrNoCourses means the HTML parsed but held no course rows — usually a sign the
// fetch landed on the login page instead of the dashboard (bad creds / session).
var ErrNoCourses = errors.New("portal: no course rows found (likely not a logged-in dashboard)")

// ParseDashboard turns dashboard HTML into a Dashboard. It returns ErrNoCourses
// if no courses are found so callers can distinguish a failed login from a parse
// bug.
func ParseDashboard(html string) (*Dashboard, error) {
	d := &Dashboard{}
	if m := reUSN.FindStringSubmatch(html); m != nil {
		d.USN = strings.TrimSpace(m[1])
	}
	if m := reName.FindStringSubmatch(html); m != nil {
		d.Name = strings.TrimSpace(m[1])
	}
	if m := reBranch.FindStringSubmatch(html); m != nil {
		d.Branch = strings.TrimSpace(m[1])
		d.Sem = m[2]
		d.Section = m[3]
	}
	if m := reUpdated.FindStringSubmatch(html); m != nil {
		d.LastUpdated = strings.TrimSpace(m[1])
	}

	details := detailsByCode(html)

	for _, row := range reTableRow.FindAllStringSubmatch(reComment.ReplaceAllString(html, ""), -1) {
		cells := reTD.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 8 || strings.Contains(cells[0][1], "top_back") {
			continue // not a course row (the per-course detail blocks start with a top_back code cell)
		}
		text := func(i int) string { return html2text(cells[i][2]) }
		c := Course{
			Code: text(0), Name: text(1), LTPS: text(2), Credits: num(text(3)),
			Type: text(4), Nature: text(5), CIEMax: num(text(6)), SEEMax: num(text(7)),
		}
		if !reCode.MatchString(c.Code) {
			continue
		}
		if l := reListLink.FindStringSubmatch(row[1]); l != nil {
			c.CourseID, c.SecID, c.SemID = l[1], l[2], l[3] // no link: the course still shows, without a class list
		}
		if det, ok := details[c.Code]; ok {
			c.Attended, c.Conducted = det.attended, det.conducted
			c.CIE, c.CIETotal = det.cie, det.cieTotal
		}
		d.Courses = append(d.Courses, c)
	}
	if len(d.Courses) == 0 {
		return nil, ErrNoCourses
	}
	return d, nil
}

type detail struct {
	attended, conducted int
	cie                 []CIEPart
	cieTotal            float64
}

// nameActivities fixes the dashboard's habit of printing a cie and an activity
// under the same name (SE1, SE1, SE2, SE2). The first SEn is cie n and the next
// is activity n (checked against the CIE page: CIE1, A1, CIE2, A2), so a repeat
// becomes "A<n>". A name that appears once is left alone.
//
// The see is only ever in exam history, never on the dashboard, so a test
// named like it (cie3 showed up as one) is the next cie, whatever its number.
func nameActivities(parts []CIEPart) {
	seen := map[string]bool{}
	cies := 0
	for i, p := range parts {
		if reSEEName.MatchString(p.Name) {
			cies++
			parts[i].Name = "SE" + strconv.Itoa(cies)
			seen[parts[i].Name] = true
			continue
		}
		if !seen[p.Name] {
			cies++
			seen[p.Name] = true
			continue
		}
		if n := strings.TrimLeft(p.Name, "SEsecieCIE \t"); n != "" {
			parts[i].Name = "A" + n
		}
	}
}

// detailsByCode scans each per-course detail block (headed by its code in a
// `top_back` cell) for its CIE parts, CIE total, and "Present a/c" pair.
func detailsByCode(html string) map[string]detail {
	out := map[string]detail{}
	idx := reDetailCode.FindAllStringSubmatchIndex(html, -1)
	for i, loc := range idx {
		code := strings.TrimSpace(html[loc[2]:loc[3]])
		end := len(html)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		block := html[loc[1]:end]
		var det detail

		marks, _, _ := strings.Cut(block, "Attendance Details") // CIE sits above attendance
		for _, m := range reCIEPart.FindAllStringSubmatch(marks, -1) {
			part := CIEPart{Name: strings.TrimSpace(m[1]), Entered: strings.TrimSpace(m[2]), Max: num(m[3])}
			switch got := strings.TrimSpace(m[4]); {
			case strings.EqualFold(got, "A") || strings.EqualFold(got, "AB"):
				part.Absent = true
			case got == "" || got == "-":
				part.Pending = true
			default:
				if v, err := strconv.ParseFloat(got, 64); err == nil {
					part.Obtained = v
				} else {
					part.Pending = true
				}
			}
			det.cie = append(det.cie, part)
		}
		nameActivities(det.cie)
		if m := reCIETotal.FindStringSubmatch(marks); m != nil {
			det.cieTotal = num(m[1])
		}

		if p := strings.Index(block, "Present"); p >= 0 {
			if m := rePresentPair.FindStringSubmatch(block[p:]); m != nil {
				det.attended, _ = strconv.Atoi(m[1])
				det.conducted, _ = strconv.Atoi(m[2])
			}
		}
		out[code] = det
	}
	return out
}

// html2text is a cell's text: tags dropped, entities decoded, spaces collapsed.
func html2text(s string) string {
	return strings.TrimSpace(reSpaces.ReplaceAllString(stdhtml.UnescapeString(reTag.ReplaceAllString(s, " ")), " "))
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}
