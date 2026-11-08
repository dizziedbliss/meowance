package portal

import (
	"regexp"
	"strings"
)

// TimetablePath is the portal's "this week" timetable, relative to the login page URL.
const TimetablePath = "?option=com_studentdashboard&controller=studentdashboard&task=timetable"

// Class is one slot in the week. The faculty column is deliberately never parsed:
// no lecturer names in this app.
type Class struct {
	Start string `json:"start"` // "09:30"
	End   string `json:"end"`   // "10:30"
	Code  string `json:"code"`  // "23AI704D"
	Name  string `json:"name"`  // as printed (the portal uses caps here)
	Room  string `json:"room"`  // "CLASS ROOM", "LABORATARY" (sic)
	Batch string `json:"batch"` // "BATCH 1" or ""
}

// Day is one weekday's classes, with the date the portal printed (dd-mm-yyyy).
type Day struct {
	Weekday string  `json:"weekday"` // "Monday"
	Date    string  `json:"date"`    // "02-11-2026"
	Classes []Class `json:"classes"`
}

var (
	reDayCaption = regexp.MustCompile(`<caption>\s*Timetable\s*<span[^>]*>\s*([A-Za-z]+)\s*</span>\s*<span[^>]*>\s*([0-9-]+)\s*</span>\s*</caption>`)
	reSlot       = regexp.MustCompile(`(?s)<tr>\s*<td[^>]*>\s*(\d{1,2}:\d{2})\s*-\s*(\d{1,2}:\d{2})\s*</td>\s*` +
		`<td[^>]*>\s*([^<]*?)\s*</td>\s*<td[^>]*>[^<]*</td>\s*<td[^>]*>\s*([^<]*?)\s*</td>\s*<td[^>]*>\s*([^<]*?)\s*</td>`)
)

// ParseTimetable reads the week shown on the portal's timetable page. A page with
// no day tables gives an empty slice, not an error (a holiday week looks like that).
func ParseTimetable(html string) []Day {
	idx := reDayCaption.FindAllStringSubmatchIndex(html, -1)
	days := make([]Day, 0, len(idx))
	for i, loc := range idx {
		end := len(html)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		day := Day{Weekday: html[loc[2]:loc[3]], Date: html[loc[4]:loc[5]], Classes: []Class{}}
		for _, m := range reSlot.FindAllStringSubmatch(html[loc[1]:end], -1) {
			code, name, _ := strings.Cut(m[3], " - ")
			day.Classes = append(day.Classes, Class{
				Start: m[1], End: m[2],
				Code: strings.TrimSpace(code), Name: strings.TrimSpace(name),
				Room: m[4], Batch: m[5],
			})
		}
		days = append(days, day)
	}
	return days
}
