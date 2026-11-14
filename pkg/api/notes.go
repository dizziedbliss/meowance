package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dizziedbliss/meowance/pkg/notes"
)

// NotesDeps is everything the notes endpoint needs, built once per warm instance.
type NotesDeps struct {
	Svc     *notes.Service
	Signer  *notes.Signer
	Portal  Fetcher  // verifies a student, exactly like the attendance login
	Origins []string // page origins allowed to upload straight to the store
}

// ReportNotifier emails Max a report through the feedback mailer, or is nil without one.
func ReportNotifier(m Mailer) func(context.Context, notes.ReportInfo) error {
	if m == nil {
		return nil
	}
	return func(ctx context.Context, r notes.ReportInfo) error {
		return m.Send(ctx, r.Reporter, fmt.Sprintf("REPORTED NOTE %s (%q, course %s): %s", r.NoteID, r.Title, r.Code, r.Reason))
	}
}

var reNotesCode = regexp.MustCompile(`^[A-Za-z0-9]{3,12}$`)

// notesReq is the POST body; the fields each action reads are noted in notesWrite.
type notesReq struct {
	Action string `json:"action"`
	USN    string `json:"usn"`
	DOB    string `json:"dob"`
	Title  string `json:"title"`
	Code   string `json:"code"`
	Sem    string `json:"sem"`
	MIME   string `json:"mime"`
	Size   int64  `json:"size"`
	NoteID string `json:"noteId"`
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// Notes returns the /api/notes handler:
//
//	GET  ?code=23AI501   a course's notes (no login)
//	GET  ?file=<id>      302 to the note's file (no login)
//	POST login           USN + birthday checked on the portal → a 7-day token (the same
//	                     per-instance wrong-birthday guard as /api/attendance, but its own counts)
//	POST init|commit|vote|report   with "Authorization: Bearer <token>"
//
// get builds the dependencies lazily; when it fails (missing env) every call is 503 notes_off.
func Notes(get func() (*NotesDeps, error)) http.HandlerFunc {
	return notesHandler(get, newLimiter(time.Now))
}

func notesHandler(get func() (*NotesDeps, error), lim *limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		d, err := get()
		if err != nil {
			log.Printf("notes unavailable: %v", err)
			writeErr(w, http.StatusServiceUnavailable, CodeNotesOff)
			return
		}
		switch r.Method {
		case http.MethodGet:
			notesRead(w, r, d)
		case http.MethodPost:
			notesWrite(w, r, d, lim)
		default:
			w.Header().Set("Allow", "GET, POST")
			writeErr(w, http.StatusMethodNotAllowed, CodeBadInput)
		}
	}
}

func notesRead(w http.ResponseWriter, r *http.Request, d *NotesDeps) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	q := r.URL.Query()
	switch {
	case q.Get("code") != "":
		if !reNotesCode.MatchString(q.Get("code")) {
			writeErr(w, http.StatusBadRequest, CodeBadInput)
			return
		}
		views, err := d.Svc.List(ctx, q.Get("code"))
		if err != nil {
			notesErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"notes": views})
	case q.Get("file") != "":
		if len(q.Get("file")) > 40 {
			writeErr(w, http.StatusNotFound, CodeNotFound)
			return
		}
		link, err := d.Svc.FileLink(ctx, q.Get("file"))
		if err != nil {
			notesErr(w, err)
			return
		}
		http.Redirect(w, r, link, http.StatusFound)
	default:
		writeErr(w, http.StatusBadRequest, CodeBadInput)
	}
}

func notesWrite(w http.ResponseWriter, r *http.Request, d *NotesDeps, lim *limiter) {
	var req notesReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, CodeBadInput)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	if req.Action == "login" { // usn, dob
		usn, dob, want, ok := validate(Request{USN: req.USN, DOB: req.DOB}, time.Now())
		if !ok {
			writeErr(w, http.StatusBadRequest, CodeBadInput)
			return
		}
		if _, fail := portalLogin(r, d.Portal, lim, usn, dob, want); fail != nil {
			fail.write(w)
			return
		}
		tok, err := d.Signer.Issue(usn)
		if err != nil {
			notesErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"token": tok, "expiresIn": int(notes.TokenLife.Seconds())})
		return
	}

	if !slices.Contains([]string{"init", "commit", "vote", "report"}, req.Action) {
		writeErr(w, http.StatusBadRequest, CodeBadInput)
		return
	}
	usn, ok := d.Signer.Verify(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if !ok {
		writeErr(w, http.StatusUnauthorized, CodeBadToken)
		return
	}
	switch req.Action {
	case "init": // title, code, sem, mime, size; the Origin header says which page may upload
		origin := r.Header.Get("Origin")
		if origin == "" {
			writeErr(w, http.StatusBadRequest, CodeBadInput)
			return
		}
		if !slices.Contains(d.Origins, origin) {
			writeErr(w, http.StatusForbidden, CodeForbidden)
			return
		}
		res, err := d.Svc.Init(ctx, usn, notes.InitInput{Title: req.Title, Code: req.Code, Sem: req.Sem, MIME: req.MIME, Size: req.Size, Origin: origin})
		if err != nil {
			notesErr(w, err)
			return
		}
		writeJSON(w, map[string]string{"noteId": res.NoteID, "session": res.Session})
	case "commit": // noteId, key
		n, err := d.Svc.Commit(ctx, usn, req.NoteID, req.Key)
		if err != nil {
			notesErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"note": notes.ViewOf(n)})
	case "vote": // noteId
		if err := d.Svc.Vote(ctx, usn, req.NoteID); err != nil {
			notesErr(w, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	case "report": // noteId, reason
		if err := d.Svc.Report(ctx, usn, req.NoteID, req.Reason); err != nil {
			notesErr(w, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		writeErr(w, http.StatusBadRequest, CodeBadInput)
	}
}

// notesErr turns a service error into the status and code the UI maps to words.
func notesErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, notes.ErrBadInput), errors.Is(err, notes.ErrBadUpload):
		writeErr(w, http.StatusBadRequest, CodeBadInput)
	case errors.Is(err, notes.ErrTooMany):
		writeErr(w, http.StatusTooManyRequests, CodeTooManyUploads)
	case errors.Is(err, notes.ErrNotFound):
		writeErr(w, http.StatusNotFound, CodeNotFound)
	case errors.Is(err, notes.ErrNotYours):
		writeErr(w, http.StatusForbidden, CodeForbidden)
	case errors.Is(err, notes.ErrRejected):
		writeErr(w, http.StatusUnprocessableEntity, CodeRejected)
	default:
		log.Printf("notes failed: %v", err)
		writeErr(w, http.StatusBadGateway, CodeStorage)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
