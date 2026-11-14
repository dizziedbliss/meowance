package notes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// memRepo is an in-memory Repo.
type memRepo struct {
	mu      sync.Mutex
	notes   map[string]*Note
	votes   map[string]map[string]bool
	reports map[string]map[string]string
}

func newMemRepo() *memRepo {
	return &memRepo{notes: map[string]*Note{}, votes: map[string]map[string]bool{}, reports: map[string]map[string]string{}}
}

func (r *memRepo) CreateWithin(_ context.Context, n Note, since time.Time, max int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := 0
	for _, o := range r.notes {
		if o.Uploader == n.Uploader && !o.Created.Before(since) {
			c++
		}
	}
	if c >= max {
		return false, nil
	}
	r.notes[n.ID] = &n
	return true, nil
}
func (r *memRepo) Reject(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.notes[id]
	if !ok {
		return ErrNotFound
	}
	n.Status = "rejected"
	return nil
}
func (r *memRepo) Get(_ context.Context, id string) (Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.notes[id]
	if !ok {
		return Note{}, ErrNotFound
	}
	c := *n
	c.Votes = len(r.votes[id])
	return c, nil
}
func (r *memRepo) SetLive(_ context.Context, id, key string, size int64, mime string) error {
	n, ok := r.notes[id]
	if !ok {
		return ErrNotFound
	}
	n.Status, n.FileKey, n.Size, n.MIME = "live", key, size, mime
	return nil
}
func (r *memRepo) Delete(_ context.Context, id string) error { delete(r.notes, id); return nil }
func (r *memRepo) ListLive(_ context.Context, code string) ([]Note, error) {
	var out []Note
	for id, n := range r.notes {
		if n.Status == "live" && n.Code == code {
			c := *n
			c.Votes = len(r.votes[id])
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}
func (r *memRepo) Vote(_ context.Context, id, usn string) (bool, error) {
	if r.votes[id] == nil {
		r.votes[id] = map[string]bool{}
	}
	if r.votes[id][usn] {
		return false, nil
	}
	r.votes[id][usn] = true
	return true, nil
}
func (r *memRepo) Report(_ context.Context, id, usn, reason string) (bool, error) {
	if r.reports[id] == nil {
		r.reports[id] = map[string]string{}
	}
	if _, dup := r.reports[id][usn]; dup {
		return false, nil
	}
	r.reports[id][usn] = reason
	return true, nil
}

// fakeStore records what the service asks of a Store.
type fakeStore struct {
	mu        sync.Mutex
	discarded []Upload
	opened    []Upload
	finish    []Upload
	deleted   []string
	newErr    error
	finErr    error
}

func (s *fakeStore) NewUpload(_ context.Context, u Upload) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.newErr != nil {
		return "", s.newErr
	}
	s.opened = append(s.opened, u)
	return "https://upload.example/" + u.NoteID, nil
}
func (s *fakeStore) Finish(_ context.Context, u Upload, key string) (Stored, error) {
	s.finish = append(s.finish, u)
	if s.finErr != nil {
		return Stored{}, s.finErr
	}
	return Stored{Key: key, Size: u.Size, MIME: u.MIME}, nil
}
func (s *fakeStore) Discard(_ context.Context, u Upload) error {
	s.discarded = append(s.discarded, u)
	return nil
}
func (s *fakeStore) Link(key string) string { return "https://files.example/" + key }
func (s *fakeStore) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

type harness struct {
	svc      *Service
	repo     *memRepo
	store    *fakeStore
	now      time.Time
	reported []ReportInfo
	n        int
	idMu     sync.Mutex
}

func newHarness() *harness {
	h := &harness{repo: newMemRepo(), store: &fakeStore{}, now: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	h.svc = &Service{
		Repo: h.repo, Store: h.store,
		NewID:  func() string { h.idMu.Lock(); defer h.idMu.Unlock(); h.n++; return fmt.Sprintf("n%d", h.n) },
		Now:    func() time.Time { return h.now },
		Notify: func(_ context.Context, r ReportInfo) error { h.reported = append(h.reported, r); return nil },
	}
	return h
}

func in() InitInput {
	return InitInput{Title: "Unit 3 notes", Code: "23ai501", Sem: "5", MIME: "application/pdf", Size: 2048, Origin: "https://meowance.vercel.app"}
}

const usn = "4MC23AI001"

// goLive runs a note through init and commit.
func (h *harness) goLive(t *testing.T, owner string) string {
	t.Helper()
	r, err := h.svc.Init(context.Background(), owner, in())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Commit(context.Background(), owner, r.NoteID, "key-"+r.NoteID); err != nil {
		t.Fatal(err)
	}
	return r.NoteID
}

func TestInitOpensAPendingNote(t *testing.T) {
	h := newHarness()
	r, err := h.svc.Init(context.Background(), usn, in())
	if err != nil || r.NoteID != "n1" || r.Session != "https://upload.example/n1" {
		t.Fatalf("Init = %+v, %v", r, err)
	}
	n := h.repo.notes["n1"]
	if n.Status != "pending" || n.Uploader != usn || n.Dept != "AI" || n.Code != "23AI501" || n.Sem != "5" || n.Storage != "gdrive" {
		t.Errorf("pending note = %+v", n)
	}
	want := Upload{NoteID: "n1", MIME: "application/pdf", Size: 2048, Origin: "https://meowance.vercel.app", Dept: "AI", Sem: "5", Code: "23AI501"}
	if len(h.store.opened) != 1 || h.store.opened[0] != want {
		t.Errorf("store got %+v, want %+v", h.store.opened, want)
	}
}

func TestInitRejectsBadInputBeforeTouchingAnything(t *testing.T) {
	bad := map[string]func(*InitInput){
		"no title":      func(i *InitInput) { i.Title = "  " },
		"long title":    func(i *InitInput) { i.Title = strings.Repeat("a", 81) },
		"control chars": func(i *InitInput) { i.Title = "hi\x00there" },
		"bad code":      func(i *InitInput) { i.Code = "23 AI;501" },
		"empty code":    func(i *InitInput) { i.Code = "" },
		"sem 0":         func(i *InitInput) { i.Sem = "0" },
		"sem 9":         func(i *InitInput) { i.Sem = "9" },
		"sem word":      func(i *InitInput) { i.Sem = "five" },
	}
	for name, mut := range bad {
		h := newHarness()
		i := in()
		mut(&i)
		if _, err := h.svc.Init(context.Background(), usn, i); !errors.Is(err, ErrBadInput) {
			t.Errorf("%s: err = %v, want ErrBadInput", name, err)
		}
		if len(h.repo.notes) != 0 || len(h.store.opened) != 0 {
			t.Errorf("%s: touched the repo or store", name)
		}
	}
	h := newHarness()
	if _, err := h.svc.Init(context.Background(), "x", in()); !errors.Is(err, ErrBadInput) {
		t.Errorf("a malformed USN should be ErrBadInput, got %v", err)
	}
}

func TestInitDailyLimit(t *testing.T) {
	h := newHarness()
	for i := 0; i < DailyUploads; i++ {
		if _, err := h.svc.Init(context.Background(), usn, in()); err != nil {
			t.Fatalf("upload %d: %v", i+1, err)
		}
	}
	if _, err := h.svc.Init(context.Background(), usn, in()); !errors.Is(err, ErrTooMany) {
		t.Errorf("over the limit: err = %v, want ErrTooMany", err)
	}
	if _, err := h.svc.Init(context.Background(), "4MC23AI002", in()); err != nil {
		t.Errorf("someone else is not limited by this USN: %v", err)
	}
	h.now = h.now.Add(25 * time.Hour)
	if _, err := h.svc.Init(context.Background(), usn, in()); err != nil {
		t.Errorf("the limit should reset after a day: %v", err)
	}
}

func TestInitCleansUpWhenTheStoreFails(t *testing.T) {
	h := newHarness()
	h.store.newErr = errors.New("drive down")
	if _, err := h.svc.Init(context.Background(), usn, in()); err == nil {
		t.Fatal("store failure was swallowed")
	}
	if len(h.repo.notes) != 0 {
		t.Errorf("a pending note with no upload was left behind: %v", h.repo.notes)
	}
}

func TestCommitMakesTheNoteLive(t *testing.T) {
	h := newHarness()
	r, _ := h.svc.Init(context.Background(), usn, in())
	n, err := h.svc.Commit(context.Background(), usn, r.NoteID, "drivekey1")
	if err != nil {
		t.Fatal(err)
	}
	if n.Status != "live" || n.FileKey != "drivekey1" || n.Size != 2048 {
		t.Errorf("committed = %+v", n)
	}
	if len(h.store.finish) != 1 || h.store.finish[0].NoteID != r.NoteID || h.store.finish[0].Code != "23AI501" {
		t.Errorf("Finish got %+v: must check the announced upload", h.store.finish)
	}
}

func TestCommitIsOnlyForTheUploaderAndOnlyOnce(t *testing.T) {
	h := newHarness()
	r, _ := h.svc.Init(context.Background(), usn, in())
	if _, err := h.svc.Commit(context.Background(), "4MC23AI002", r.NoteID, "k"); !errors.Is(err, ErrNotYours) {
		t.Errorf("another student: err = %v, want ErrNotYours", err)
	}
	if _, err := h.svc.Commit(context.Background(), usn, "nope", "k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown note: err = %v, want ErrNotFound", err)
	}
	if _, err := h.svc.Commit(context.Background(), usn, r.NoteID, "k"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Commit(context.Background(), usn, r.NoteID, "k2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("committing twice: err = %v, want ErrNotFound (it's no longer pending)", err)
	}
}

func TestRejectedUploadIsDiscardedButStillCounts(t *testing.T) {
	h := newHarness()
	r, _ := h.svc.Init(context.Background(), usn, in())
	h.store.finErr = ErrRejected
	if _, err := h.svc.Commit(context.Background(), usn, r.NoteID, "k"); !errors.Is(err, ErrRejected) {
		t.Fatalf("err = %v, want ErrRejected", err)
	}
	if n := h.repo.notes[r.NoteID]; n == nil || n.Status != "rejected" {
		t.Fatalf("note = %+v: a rejected upload must stay on record so it counts toward the limit", n)
	}
	if len(h.store.discarded) != 1 || h.store.discarded[0].NoteID != r.NoteID {
		t.Errorf("discarded = %+v: the stray file in the store should be removed", h.store.discarded)
	}
	if len(h.store.deleted) != 0 {
		t.Errorf("deleted %v: never delete the key the client sent, it may be someone else's file", h.store.deleted)
	}
	if _, err := h.svc.Commit(context.Background(), usn, r.NoteID, "k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a rejected note can't be committed again: %v", err)
	}
	if got, _ := h.svc.List(context.Background(), "23AI501"); len(got) != 0 {
		t.Errorf("a rejected note is listed: %+v", got)
	}
}

func TestFailedUploadsCannotBeRetriedForever(t *testing.T) {
	h := newHarness()
	h.store.finErr = ErrRejected
	for i := 0; i < DailyUploads; i++ {
		r, err := h.svc.Init(context.Background(), usn, in())
		if err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
		h.svc.Commit(context.Background(), usn, r.NoteID, "k")
	}
	if _, err := h.svc.Init(context.Background(), usn, in()); !errors.Is(err, ErrTooMany) {
		t.Errorf("after %d failed uploads: err = %v, want ErrTooMany", DailyUploads, err)
	}
}

func TestParallelInitsCannotRaceThePastTheLimit(t *testing.T) {
	h := newHarness()
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.svc.Init(context.Background(), usn, in()); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != DailyUploads {
		t.Errorf("%d of 40 parallel uploads got through, want exactly %d", ok, DailyUploads)
	}
}

func TestListShowsLiveNotesOfACourseWithLabelsNotNames(t *testing.T) {
	h := newHarness()
	a := h.goLive(t, usn)
	h.now = h.now.Add(time.Hour)
	b := h.goLive(t, "4MC23CS002")
	h.svc.Init(context.Background(), usn, in()) // a pending one must not show up
	other := in()
	other.Code = "23AI502"
	r, _ := h.svc.Init(context.Background(), usn, other)
	h.svc.Commit(context.Background(), usn, r.NoteID, "k") // a different course

	got, err := h.svc.List(context.Background(), "23ai501")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != b || got[1].ID != a {
		t.Fatalf("List = %+v, want newest first: %s then %s", got, b, a)
	}
	if got[1].Label != Label(a, "AI") || got[0].Label != Label(b, "CS") {
		t.Errorf("labels = %q, %q", got[1].Label, got[0].Label)
	}
	if fmt.Sprintf("%+v", got) != strings.ReplaceAll(fmt.Sprintf("%+v", got), usn, "") {
		t.Error("a USN leaked into the list")
	}
}

func TestVoteCountsOncePerStudent(t *testing.T) {
	h := newHarness()
	id := h.goLive(t, usn)
	for _, who := range []string{"4MC23AI002", "4MC23AI002", "4MC23CS003"} {
		if err := h.svc.Vote(context.Background(), who, id); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := h.svc.List(context.Background(), "23AI501")
	if got[0].Votes != 2 {
		t.Errorf("votes = %d, want 2 (a repeat vote doesn't count)", got[0].Votes)
	}
	if err := h.svc.Vote(context.Background(), usn, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("vote on a missing note: %v", err)
	}
	r, _ := h.svc.Init(context.Background(), usn, in())
	if err := h.svc.Vote(context.Background(), usn, r.NoteID); !errors.Is(err, ErrNotFound) {
		t.Errorf("vote on a pending note: %v, want ErrNotFound", err)
	}
}

func TestReportTellsMaxOncePerStudent(t *testing.T) {
	h := newHarness()
	id := h.goLive(t, usn)
	if err := h.svc.Report(context.Background(), "4MC23AI002", id, "  this is a textbook  "); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Report(context.Background(), "4MC23AI002", id, "again"); err != nil {
		t.Fatal(err)
	}
	if len(h.reported) != 1 || h.reported[0].NoteID != id || h.reported[0].Reason != "this is a textbook" || h.reported[0].Title != "Unit 3 notes" {
		t.Errorf("reported = %+v, want one trimmed report", h.reported)
	}
	for name, reason := range map[string]string{"empty": "   ", "too long": strings.Repeat("x", 301)} {
		if err := h.svc.Report(context.Background(), "4MC23CS003", id, reason); !errors.Is(err, ErrBadInput) {
			t.Errorf("%s reason: err = %v, want ErrBadInput", name, err)
		}
	}
	if err := h.svc.Report(context.Background(), usn, "nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("report on a missing note: %v", err)
	}
}

func TestFileLinkOnlyForLiveNotes(t *testing.T) {
	h := newHarness()
	id := h.goLive(t, usn)
	if got, err := h.svc.FileLink(context.Background(), id); err != nil || got != "https://files.example/key-"+id {
		t.Errorf("FileLink = %q, %v", got, err)
	}
	r, _ := h.svc.Init(context.Background(), usn, in())
	for _, bad := range []string{r.NoteID, "nope"} {
		if _, err := h.svc.FileLink(context.Background(), bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("FileLink(%q) err = %v, want ErrNotFound", bad, err)
		}
	}
}

func TestRemoveDeletesTheFileAndTheRow(t *testing.T) {
	h := newHarness()
	id := h.goLive(t, usn)
	if err := h.svc.Remove(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if len(h.store.deleted) != 1 || h.store.deleted[0] != "key-"+id {
		t.Errorf("store deletes = %v", h.store.deleted)
	}
	if _, ok := h.repo.notes[id]; ok {
		t.Error("row still there")
	}
	if err := h.svc.Remove(context.Background(), id); !errors.Is(err, ErrNotFound) {
		t.Errorf("removing twice: %v, want ErrNotFound", err)
	}
}
