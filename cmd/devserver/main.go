// Command devserver runs POST /api/attendance locally for the Vite dev server
// (which proxies /api here).
//
//	go run ./cmd/devserver                      # talks to the real portal
//	go run ./cmd/devserver -fixture <file.html> # serves a saved dashboard for any login
//
// In fixture mode, a birthday of 01012000 simulates a wrong login.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dizziedbliss/meowance/pkg/api"
	"github.com/dizziedbliss/meowance/pkg/portal"
)

// fixture serves saved pages. The attendance list and CIE page are one course's;
// they're served for every course code asked for.
type fixture struct{ dashboard, timetable, calendar, attendanceList, cieDetails, examHistory string }

func (f fixture) Fetch(ctx context.Context, usn string, dob time.Time, want portal.Extras) (*portal.Snapshot, error) {
	select {
	case <-time.After(600 * time.Millisecond): // feel like a real round trip
	case <-ctx.Done():
		return nil, portal.ErrPortalDown
	}
	if dob.Format("02012006") == "01012000" {
		return nil, portal.ErrBadLogin
	}
	d, err := portal.ParseDashboard(f.dashboard)
	if err != nil {
		return nil, err
	}
	s := &portal.Snapshot{Dashboard: d, Asked: want}
	if want.Timetable && f.timetable != "" {
		s.Timetable = portal.ParseTimetable(f.timetable)
	}
	if want.Calendar && f.calendar != "" {
		s.Calendar = portal.ParseCalendar(f.calendar)
	}
	if want.Exams {
		s.Exams, _ = portal.ParseExamHistory(f.examHistory)
	}
	for _, code := range want.History {
		if s.History == nil {
			s.History = map[string]*portal.CourseHistory{}
		}
		h, _ := portal.ParseAttendanceList(f.attendanceList)
		if h != nil {
			h.Code = code
		}
		s.History[code] = h
	}
	for _, code := range want.CIE {
		if s.CIE == nil {
			s.CIE = map[string]*portal.CIEDetails{}
		}
		x, _ := portal.ParseCIEDetails(f.cieDetails)
		if x != nil {
			x.Code = code
		}
		s.CIE[code] = x
	}
	return s, nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	fx := flag.String("fixture", "", "serve this saved dashboard HTML instead of the real portal")
	flag.Parse()

	client := portal.NewClient()
	client.Logf = log.Printf // per-page status for debugging; never credentials or URLs
	var f api.Fetcher = client
	if *fx == "" {
		log.Printf("real portal, residential proxy: %t (OXYLABS_USERNAME/PASSWORD)", client.Proxied())
	}
	if *fx != "" {
		b, err := os.ReadFile(*fx)
		if err != nil {
			log.Fatal(err)
		}
		// timetable.html and calendar.html next to the dashboard fixture are picked up automatically
		dir := filepath.Dir(*fx)
		tt, _ := os.ReadFile(filepath.Join(dir, "timetable.html"))
		cal, _ := os.ReadFile(filepath.Join(dir, "calendar.html"))
		al, _ := os.ReadFile(filepath.Join(dir, "attendancelist.html"))
		cie, _ := os.ReadFile(filepath.Join(dir, "ciedetails.html"))
		ex, _ := os.ReadFile(filepath.Join(dir, "examhistory.html"))
		f = fixture{dashboard: string(b), timetable: string(tt), calendar: string(cal),
			attendanceList: string(al), cieDetails: string(cie), examHistory: string(ex)}
		log.Printf("fixture mode: %s (timetable: %t, calendar: %t)", *fx, len(tt) > 0, len(cal) > 0)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/attendance", api.Attendance(f))
	log.Printf("listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
