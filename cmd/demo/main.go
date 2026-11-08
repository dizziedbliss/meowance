// Command demo parses a saved Contineo dashboard HTML file and prints each
// course's attendance plus the bunk analysis at 75% and 85%. It's a visual
// sanity check for the parser + math — not part of the shipped app.
//
// Usage: go run ./cmd/demo <dashboard.html>
package main

import (
	"fmt"
	"os"

	"github.com/dizziedbliss/meowance/internal/attend"
	"github.com/dizziedbliss/meowance/internal/portal"
)

func main() {
	path := "internal/portal/testdata/dashboard.html"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
	d, err := portal.ParseDashboard(string(b))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}

	fmt.Printf("%s  (%s)  %s SEM %s SEC %s\n", d.Name, d.USN, d.Branch, d.Sem, d.Section)
	fmt.Printf("Last updated on the portal: %s\n\n", d.LastUpdated)

	fmt.Printf("%-10s %-34s %3s %5s   %-20s   %-20s\n", "CODE", "COURSE", "Cr", "A/C", "@75%", "@85%")
	fmt.Println(line(126))
	for _, c := range d.Courses {
		pct := attend.Ratio(c.Attended, c.Conducted) * 100
		fmt.Printf("%-10s %-34s %3.0f %5s   %-20s   %-20s\n",
			c.Code, trunc(c.Name, 34), c.Credits,
			fmt.Sprintf("%d/%d", c.Attended, c.Conducted),
			verdict(c.Attended, c.Conducted, 0.75),
			verdict(c.Attended, c.Conducted, 0.85),
		)
		_ = pct
	}
}

func verdict(att, con int, target float64) string {
	s := attend.At(att, con, target)
	if s.Below {
		return fmt.Sprintf("below · attend %d", s.Recover)
	}
	return fmt.Sprintf("skip up to %d", s.CanBunk)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func line(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '-'
	}
	return string(b)
}
