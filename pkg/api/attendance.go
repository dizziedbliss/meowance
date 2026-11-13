// Package api is the HTTP layer: POST /api/attendance takes a USN + date of birth,
// fetches that student's dashboard from the portal and returns it as JSON with the
// bunk math already done. Nothing is stored or logged.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dizziedbliss/meowance/pkg/attend"
	"github.com/dizziedbliss/meowance/pkg/portal"
)

// Fetcher is what the handler needs from the portal (a real *portal.Client, or a fixture in dev).
type Fetcher interface {
	Fetch(ctx context.Context, usn string, dob time.Time) (*portal.Snapshot, error)
}

// Request is the JSON body: dob is DDMMYYYY, matching how students type it.
type Request struct {
	USN string `json:"usn"`
	DOB string `json:"dob"`
}

// Student is who's logged in. Dept is the two-letter branch code inside the USN (CI, CS, VL…).
type Student struct {
	USN         string `json:"usn"`
	Name        string `json:"name"`
	Dept        string `json:"dept"`
	Sem         string `json:"sem"`
	Section     string `json:"section"`
	LastUpdated string `json:"lastUpdated"`
}

// Course carries the raw counts plus the three numbers the UI needs.
type Course struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Credits   float64 `json:"credits"`
	Attended  int     `json:"attended"`
	Conducted int     `json:"conducted"`
	Percent   float64 `json:"percent"` // one decimal
	Need75    int     `json:"need75"`  // classes in a row to get back to 75 (0 if there)
	Need85    int     `json:"need85"`  // classes in a row to get back to 85 (0 if there)
	Skip85    int     `json:"skip85"`  // classes you can miss and stay at or above 85

	CIE      []portal.CIEPart `json:"cie"`      // internal assessments so far, never null
	CIETotal float64          `json:"cieTotal"` // as the portal totals it
	CIEMax   float64          `json:"cieMax"`   // usually 50
}

type Response struct {
	Student Student  `json:"student"`
	Courses []Course `json:"courses"`
	// Timetable is this week's classes; null when the portal's timetable page
	// couldn't be fetched, [] for a week with no classes.
	Timetable []portal.Day `json:"timetable"`
	// Calendar is the semester's calendar of events; null when it couldn't be fetched.
	Calendar *portal.Calendar `json:"calendar"`
}

// Error codes the UI maps to messages.
const (
	CodeBadInput      = "bad_input"
	CodeBadLogin      = "bad_login"
	CodePortalDown    = "portal_down"
	CodePortalChanged = "portal_changed"
	CodeTooManyTries  = "too_many_tries"
	CodePortalBlocked = "portal_blocked"
	CodeInternal      = "internal"
)

// Only MCE students: 4MC + 2-digit year + 2-letter branch + 3-digit roll.
var reUSN = regexp.MustCompile(`^4MC[0-9]{2}[A-Z]{2}[0-9]{3}$`)

// fetchBudget is how long one login may take, all retries included.
var fetchBudget = 50 * time.Second

// Attendance returns the POST /api/attendance handler.
func Attendance(f Fetcher) http.HandlerFunc {
	return attendance(f, newLimiter(time.Now))
}

func attendance(f Fetcher, lim *limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeErr(w, http.StatusMethodNotAllowed, CodeBadInput)
			return
		}
		var req Request
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, CodeBadInput)
			return
		}
		usn, dob, ok := validate(req, time.Now())
		if !ok {
			writeErr(w, http.StatusBadRequest, CodeBadInput)
			return
		}

		// Reserve this attempt before asking the portal. Out of tries for this
		// USN (from this IP, or overall): refuse without contacting the portal.
		ip := clientIP(r)
		p, ok, wait := lim.begin(usn, ip)
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			writeErr(w, http.StatusTooManyRequests, CodeTooManyTries)
			return
		}

		// Finish before Vercel's 60s cap kills us: a cut-off function sends its own
		// 504 page and never releases the reserved try.
		ctx, cancel := context.WithTimeout(r.Context(), fetchBudget)
		snap, err := f.Fetch(ctx, usn, dob)
		cancel()
		if errors.Is(err, portal.ErrBadLogin) {
			// a wrong birthday: the reserved attempt stays counted
			writeErr(w, http.StatusUnauthorized, CodeBadLogin)
			return
		}
		lim.done(p, usn, ip, err == nil)
		if err != nil {
			log.Printf("fetch failed: %v", err)
		}
		switch {
		case err == nil:
		case errors.Is(err, portal.ErrPortalDown):
			writeErr(w, http.StatusBadGateway, CodePortalDown)
			return
		case errors.Is(err, portal.ErrForbidden):
			// the portal is blocking our IP: an upstream failure, not the student's
			writeErr(w, http.StatusBadGateway, CodePortalBlocked)
			return
		case errors.Is(err, portal.ErrPortalChanged):
			writeErr(w, http.StatusBadGateway, CodePortalChanged)
			return
		default:
			writeErr(w, http.StatusInternalServerError, CodeInternal)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Build(snap, usn))
	}
}

// validate normalises the USN and parses the DDMMYYYY birthday.
func validate(req Request, now time.Time) (string, time.Time, bool) {
	usn := strings.ToUpper(strings.TrimSpace(req.USN))
	if !reUSN.MatchString(usn) {
		return "", time.Time{}, false
	}
	dob, err := time.Parse("02012006", strings.TrimSpace(req.DOB))
	if err != nil || dob.Year() < 1970 || dob.After(now) {
		return "", time.Time{}, false
	}
	return usn, dob, true
}

// Build turns a fetched snapshot into the API response, doing the bunk math.
// usn is the validated login USN (used when the page doesn't show one).
func Build(snap *portal.Snapshot, usn string) Response {
	d := snap.Dashboard
	if d.USN != "" {
		usn = d.USN
	}
	dept := ""
	if len(usn) >= 7 {
		dept = usn[5:7]
	}
	out := Response{
		Student: Student{
			USN: usn, Name: d.Name, Dept: dept, Sem: d.Sem, Section: d.Section, LastUpdated: d.LastUpdated,
		},
		Timetable: snap.Timetable,
		Calendar:  snap.Calendar,
	}
	for _, c := range d.Courses {
		cie := c.CIE
		if cie == nil {
			cie = []portal.CIEPart{}
		}
		out.Courses = append(out.Courses, Course{
			Code:      c.Code,
			Name:      c.Name,
			Credits:   c.Credits,
			Attended:  c.Attended,
			Conducted: c.Conducted,
			Percent:   math.Round(attend.Ratio(c.Attended, c.Conducted)*1000) / 10,
			Need75:    attend.Recover(c.Attended, c.Conducted, 0.75),
			Need85:    attend.Recover(c.Attended, c.Conducted, 0.85),
			Skip85:    attend.CanBunk(c.Attended, c.Conducted, 0.85),
			CIE:       cie,
			CIETotal:  c.CIETotal,
			CIEMax:    c.CIEMax,
		})
	}
	return out
}

func writeErr(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code})
}
