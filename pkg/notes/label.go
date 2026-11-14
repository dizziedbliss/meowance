package notes

import (
	"hash/fnv"
	"strings"
)

// labelPool is what a note's uploader is called instead of their name. {dept}
// becomes the 2-letter department from the USN, in lowercase.
var labelPool = []string{
	"a very nice {dept} kid",
	"just another nerd",
	"CR???",
	"a suspiciously organised {dept} student",
	"someone who touched grass once",
	"the back bench legend",
	"a {dept} kid with colour-coded notes",
	"your friendly neighbourhood topper",
	"a sleep-deprived {dept} student",
	"a bunker turned good samaritan",
	"the one who always has a pen",
	"a {dept} kid at 3am",
	"probably a topper",
	"a mysterious cat person",
	"CR's best friend",
	"a {dept} student who actually attended",
}

// Label picks a note's anonymous uploader label. It depends only on the note's
// id, so the same note always shows the same label, and nothing about the
// uploader is stored or leaked.
func Label(noteID, dept string) string {
	h := fnv.New32a()
	h.Write([]byte(noteID))
	l := labelPool[int(h.Sum32()%uint32(len(labelPool)))]
	dept = strings.ToLower(strings.TrimSpace(dept))
	if dept == "" {
		dept = "mce"
	}
	return strings.ReplaceAll(l, "{dept}", dept)
}
