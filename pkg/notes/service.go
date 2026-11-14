package notes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// DailyUploads is how many notes one verified student can start in 24 hours.
const DailyUploads = 5

var (
	ErrBadInput = errors.New("notes: bad input")
	ErrTooMany  = errors.New("notes: too many uploads today")
	ErrNotYours = errors.New("notes: not your upload")
)

var courseCode = regexp.MustCompile(`^[A-Z0-9]{3,12}$`)

// Note is one row of notes sharing. Uploader is the USN and never leaves the server.
type Note struct {
	ID, Code, Dept, Sem, Title string
	Storage, FileKey, MIME     string
	Uploader                   string
	Status                     string // "pending" until the file is checked, then "live"
	Size                       int64
	Created                    time.Time
	Votes                      int
}

// Repo is where note rows live (Postgres in production, memory in tests).
type Repo interface {
	Create(ctx context.Context, n Note) error
	Get(ctx context.Context, id string) (Note, error) // ErrNotFound if missing; Votes filled
	SetLive(ctx context.Context, id, key string, size int64, mime string) error
	Delete(ctx context.Context, id string) error
	ListLive(ctx context.Context, code string) ([]Note, error) // newest first; Votes filled
	CountUploads(ctx context.Context, usn string, since time.Time) (int, error)
	Vote(ctx context.Context, id, usn string) (added bool, err error)
	Report(ctx context.Context, id, usn, reason string) (added bool, err error)
}

// ReportInfo is what Max is told when a student reports a note.
type ReportInfo struct {
	NoteID, Title, Code, Reason, Reporter string
}

// Service holds the rules. Callers have already proved who the student is (a
// portal login, or a valid Signer token) and pass the USN.
type Service struct {
	Repo   Repo
	Store  Store
	Notify func(ctx context.Context, r ReportInfo) error // optional
	NewID  func() string                                 // optional
	Now    func() time.Time                              // optional
}

// InitInput is what a student announces before uploading.
type InitInput struct {
	Title, Code, Sem, MIME string
	Size                   int64
	Origin                 string
}

type InitResult struct{ NoteID, Session string }

// View is a note as other students see it: a label, never who posted.
type View struct {
	ID, Title, Label, MIME string
	Size                   int64
	Votes                  int
	Created                time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) newID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Init checks the announcement, records a pending note and opens an upload session.
func (s *Service) Init(ctx context.Context, usn string, in InitInput) (InitResult, error) {
	dept, err := deptOf(usn)
	if err != nil {
		return InitResult{}, err
	}
	title := strings.TrimSpace(in.Title)
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if n := utf8.RuneCountInString(title); n < 1 || n > 80 || strings.ContainsFunc(title, unicode.IsControl) {
		return InitResult{}, fmt.Errorf("%w: title", ErrBadInput)
	}
	if !courseCode.MatchString(code) {
		return InitResult{}, fmt.Errorf("%w: course code", ErrBadInput)
	}
	if len(in.Sem) != 1 || in.Sem < "1" || in.Sem > "8" {
		return InitResult{}, fmt.Errorf("%w: semester", ErrBadInput)
	}
	id := s.newID()
	up := Upload{NoteID: id, MIME: in.MIME, Size: in.Size, Origin: in.Origin, Dept: dept, Sem: in.Sem, Code: code}
	if err := up.check(); err != nil {
		return InitResult{}, fmt.Errorf("%w: %v", ErrBadInput, err)
	}
	if in.Origin == "" {
		return InitResult{}, fmt.Errorf("%w: origin", ErrBadInput)
	}
	now := s.now()
	if n, err := s.Repo.CountUploads(ctx, usn, now.Add(-24*time.Hour)); err != nil {
		return InitResult{}, err
	} else if n >= DailyUploads {
		return InitResult{}, ErrTooMany
	}
	note := Note{ID: id, Code: code, Dept: dept, Sem: in.Sem, Title: title, Storage: "gdrive",
		MIME: in.MIME, Size: in.Size, Uploader: usn, Status: "pending", Created: now}
	if err := s.Repo.Create(ctx, note); err != nil {
		return InitResult{}, err
	}
	session, err := s.Store.NewUpload(ctx, up)
	if err != nil {
		s.Repo.Delete(ctx, id) // don't leave a note with no upload behind
		return InitResult{}, err
	}
	return InitResult{NoteID: id, Session: session}, nil
}

// Commit runs after the browser has sent the file: the store checks it is the
// announced upload, then the note goes live. A rejected upload is discarded.
func (s *Service) Commit(ctx context.Context, usn, id, key string) (Note, error) {
	n, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Note{}, err
	}
	if n.Status != "pending" {
		return Note{}, ErrNotFound
	}
	if n.Uploader != usn {
		return Note{}, ErrNotYours
	}
	up := Upload{NoteID: n.ID, MIME: n.MIME, Size: n.Size, Dept: n.Dept, Sem: n.Sem, Code: n.Code}
	stored, err := s.Store.Finish(ctx, up, key)
	if err != nil {
		if errors.Is(err, ErrRejected) || errors.Is(err, ErrNotFound) {
			s.Repo.Delete(ctx, id)
		}
		return Note{}, err
	}
	if err := s.Repo.SetLive(ctx, id, stored.Key, stored.Size, stored.MIME); err != nil {
		return Note{}, err
	}
	return s.Repo.Get(ctx, id)
}

// List is a course's live notes, newest first, with labels instead of names.
func (s *Service) List(ctx context.Context, code string) ([]View, error) {
	rows, err := s.Repo.ListLive(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, n := range rows {
		out = append(out, View{ID: n.ID, Title: n.Title, Label: Label(n.ID, n.Dept), MIME: n.MIME, Size: n.Size, Votes: n.Votes, Created: n.Created})
	}
	return out, nil
}

// Vote adds one upvote per student per live note.
func (s *Service) Vote(ctx context.Context, usn, id string) error {
	if _, err := s.live(ctx, id); err != nil {
		return err
	}
	_, err := s.Repo.Vote(ctx, id, usn)
	return err
}

// Report tells Max about a note, once per student.
func (s *Service) Report(ctx context.Context, usn, id, reason string) error {
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < 1 || n > 300 {
		return fmt.Errorf("%w: reason", ErrBadInput)
	}
	n, err := s.live(ctx, id)
	if err != nil {
		return err
	}
	added, err := s.Repo.Report(ctx, id, usn, reason)
	if err != nil || !added || s.Notify == nil {
		return err
	}
	return s.Notify(ctx, ReportInfo{NoteID: id, Title: n.Title, Code: n.Code, Reason: reason, Reporter: usn})
}

// FileLink is where a student is sent to open a live note's file.
func (s *Service) FileLink(ctx context.Context, id string) (string, error) {
	n, err := s.live(ctx, id)
	if err != nil {
		return "", err
	}
	return s.Store.Link(n.FileKey), nil
}

// Remove deletes a note and its file (Max, after a report).
func (s *Service) Remove(ctx context.Context, id string) error {
	n, err := s.Repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if n.FileKey != "" {
		if err := s.Store.Delete(ctx, n.FileKey); err != nil {
			return err
		}
	}
	return s.Repo.Delete(ctx, id)
}

func (s *Service) live(ctx context.Context, id string) (Note, error) {
	n, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Note{}, err
	}
	if n.Status != "live" {
		return Note{}, ErrNotFound
	}
	return n, nil
}

// deptOf reads the 2-letter department from an MCE USN like 4MC23AI001.
func deptOf(usn string) (string, error) {
	if len(usn) != 10 || !strings.EqualFold(usn[:3], "4MC") {
		return "", fmt.Errorf("%w: USN", ErrBadInput)
	}
	return strings.ToUpper(usn[5:7]), nil
}
