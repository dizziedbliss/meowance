package portal

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CalendarPath is the portal's calendar of events, relative to the login page URL.
const CalendarPath = "?option=com_bvbsims&controller=academiccal&task=calenderdisplay&yearId=2"

// Event kinds, taken from how the portal colours the calendar.
const (
	KindHoliday = "holiday"
	KindExam    = "exam"
	KindLastDay = "lastday"
)

// CalendarEvent is one line of the portal's events list. Dates are YYYY-MM-DD; To == From for one-day events.
type CalendarEvent struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Title string `json:"title"`
	Kind  string `json:"kind"` // holiday | exam | lastday | ""
}

// Calendar is the semester's calendar of events.
type Calendar struct {
	Start    string          `json:"start"`    // first day of the semester
	End      string          `json:"end"`      // last day of the semester
	Holidays []string        `json:"holidays"` // every day the calendar marks as a holiday, Sundays included
	ExamDays []string        `json:"examDays"`
	Events   []CalendarEvent `json:"events"`
}

var (
	reCalStart     = regexp.MustCompile(`Start Date\s*:\s*(\d{4}-\d{2}-\d{2})`)
	reCalEnd       = regexp.MustCompile(`End Date\s*:\s*(\d{4}-\d{2}-\d{2})`)
	reMonthCaption = regexp.MustCompile(`<caption>\s*<span[^>]*>\s*([A-Za-z]+)-(\d{4})\s*</span>\s*</caption>`)
	reMarkedDay    = regexp.MustCompile(`<div class='(cn-holiday|cn-minor-exam)'>\s*(\d{1,2})\s*</div>`)
	reEventLine    = regexp.MustCompile(`<p class="events-lists">\s*([^<]+?)\s*</p>`)
	// "Aug 4th to Aug 6th - Registration…" or "Aug 3rd-commencement of classes"
	reEventText = regexp.MustCompile(`^([A-Za-z]{3})[a-z]*\s+(\d{1,2})(?:st|nd|rd|th)?` +
		`(?:\s+to\s+([A-Za-z]{3})[a-z]*\s+(\d{1,2})(?:st|nd|rd|th)?)?\s*-\s*(.+)$`)
)

// ParseCalendar reads the calendar-of-events page. It returns nil if the page
// has no semester dates (not the calendar page, or the layout changed).
func ParseCalendar(html string) *Calendar {
	start := reCalStart.FindStringSubmatch(html)
	end := reCalEnd.FindStringSubmatch(html)
	if start == nil || end == nil {
		return nil
	}
	cal := &Calendar{Start: start[1], End: end[1], Holidays: []string{}, ExamDays: []string{}, Events: []CalendarEvent{}}
	holiday, exam := map[string]bool{}, map[string]bool{}

	caps := reMonthCaption.FindAllStringSubmatchIndex(html, -1)
	for i, loc := range caps {
		month, ok := monthNum(html[loc[2]:loc[3]])
		if !ok {
			continue
		}
		year, _ := strconv.Atoi(html[loc[4]:loc[5]])
		segEnd := len(html)
		if i+1 < len(caps) {
			segEnd = caps[i+1][0]
		}
		seg := html[loc[1]:segEnd] // this month's grid, then its events list

		for _, m := range reMarkedDay.FindAllStringSubmatch(seg, -1) {
			day, _ := strconv.Atoi(m[2])
			d := ymd(year, month, day)
			if m[1] == "cn-minor-exam" {
				exam[d] = true
				cal.ExamDays = append(cal.ExamDays, d)
			} else {
				holiday[d] = true
				cal.Holidays = append(cal.Holidays, d)
			}
		}
		for _, m := range reEventLine.FindAllStringSubmatch(seg, -1) {
			if ev, ok := parseEvent(m[1], year, month); ok {
				cal.Events = append(cal.Events, ev)
			}
		}
	}

	for i := range cal.Events {
		cal.Events[i].Kind = kindOf(cal.Events[i], holiday, exam)
	}
	return cal
}

func parseEvent(text string, capYear int, capMonth time.Month) (CalendarEvent, bool) {
	m := reEventText.FindStringSubmatch(strings.Join(strings.Fields(text), " "))
	if m == nil {
		return CalendarEvent{}, false
	}
	fromMonth, ok := monthNum(m[1])
	if !ok {
		return CalendarEvent{}, false
	}
	fromDay, _ := strconv.Atoi(m[2])
	toMonth, toDay := fromMonth, fromDay
	if m[3] != "" {
		if tm, ok := monthNum(m[3]); ok {
			toMonth = tm
			toDay, _ = strconv.Atoi(m[4])
		}
	}
	return CalendarEvent{
		From:  ymd(yearNear(capYear, capMonth, fromMonth), fromMonth, fromDay),
		To:    ymd(yearNear(capYear, capMonth, toMonth), toMonth, toDay),
		Title: strings.TrimSpace(m[5]),
	}, true
}

// kindOf: exam if any of its days is an exam day; holiday if any non-Sunday day
// is marked holiday; "lastday" for the last working day.
func kindOf(ev CalendarEvent, holiday, exam map[string]bool) string {
	from, err1 := time.Parse("2006-01-02", ev.From)
	to, err2 := time.Parse("2006-01-02", ev.To)
	if err1 == nil && err2 == nil && !to.Before(from) && to.Sub(from) < 62*24*time.Hour {
		isHoliday := false
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			k := d.Format("2006-01-02")
			if exam[k] {
				return KindExam
			}
			if holiday[k] && d.Weekday() != time.Sunday {
				isHoliday = true
			}
		}
		if isHoliday {
			return KindHoliday
		}
	}
	if strings.Contains(strings.ToLower(ev.Title), "last working day") {
		return KindLastDay
	}
	return ""
}

var months = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March, "apr": time.April,
	"may": time.May, "jun": time.June, "jul": time.July, "aug": time.August,
	"sep": time.September, "oct": time.October, "nov": time.November, "dec": time.December,
}

func monthNum(s string) (time.Month, bool) {
	if len(s) < 3 {
		return 0, false
	}
	m, ok := months[strings.ToLower(s[:3])]
	return m, ok
}

// yearNear handles an event line that crosses a year boundary relative to its month's caption.
func yearNear(capYear int, capMonth, m time.Month) int {
	switch diff := int(m) - int(capMonth); {
	case diff < -6:
		return capYear + 1
	case diff > 6:
		return capYear - 1
	}
	return capYear
}

func ymd(y int, m time.Month, d int) string {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}
