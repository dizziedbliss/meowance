// Package usn is the one place that knows what an MCE student's USN looks like.
package usn

import "regexp"

// Only MCE students: 4MC + 2-digit year + 2-letter branch + 3-digit roll.
var re = regexp.MustCompile(`^4MC[0-9]{2}[A-Z]{2}[0-9]{3}$`)

// Valid reports whether s is a USN, already upper-cased and trimmed.
func Valid(s string) bool { return re.MatchString(s) }

// Dept is the 2-letter branch inside a USN (CI, CS, VL, AI…), or "" when s is too short to have one.
func Dept(s string) string {
	if len(s) < 7 {
		return ""
	}
	return s[5:7]
}
