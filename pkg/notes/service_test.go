package notes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeStore records what the service asks of a Store.
type fakeStore struct {
	onNew     func() // runs inside NewUpload/Finish before they answer, e.g. to cancel the request
	onFinish  func()
	mu        sync.Mutex
	discarded []Upload
	opened    []Upload
	finish    []Upload
	deleted   []string
	newErr    error
	finErr    error
}

func (s *fakeStore) NewUpload(_ context.Context, u Upload) (string, error) {
	if s.onNew != nil {
		s.onNew()
	}
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
	if s.onFinish != nil {
		s.onFinish()
	}
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
	return InitInput{Title: "Unit 3 notes", Code: "23ai501", MIME: "application/pdf", Size: 2048, Origin: "https://meowance.vercel.app"}
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

func TestOnlyTheUploaderDeletes(t *testing.T) {
	h := newHarness()
	id := h.goLive(t, usn)
	if err := h.svc.Delete(context.Background(), "4MC23AI002", id); !errors.Is(err, ErrNotYours) {
		t.Fatalf("another student: err = %v, want ErrNotYours", err)
	}
	if len(h.store.deleted) != 0 || h.repo.notes[id] == nil {
		t.Fatal("someone else's delete touched the note")
	}
	if err := h.svc.Delete(context.Background(), usn, id); err != nil {
		t.Fatal(err)
	}
	if len(h.store.deleted) != 1 || h.repo.notes[id] == nil || h.repo.notes[id].Status == "live" {
		t.Errorf("owner delete: file deleted %v, row %+v (the row must stay, retired, so it still counts)", h.store.deleted, h.repo.notes[id])
	}
	if l, _ := h.svc.List(context.Background(), "23AI501"); len(l) != 0 {
		t.Errorf("deleted note still listed: %v", l)
	}
	if err := h.svc.Delete(context.Background(), usn, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting twice: %v, want ErrNotFound", err)
	}
}

// Deleting must not hand the daily slot back, or upload-delete loops would churn Drive forever.
func TestDeletingDoesNotRefundTheDailyLimit(t *testing.T) {
	h := newHarness()
	for i := 0; i < DailyUploads; i++ {
		id := h.goLive(t, usn)
		if err := h.svc.Delete(context.Background(), usn, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.svc.Init(context.Background(), usn, in()); !errors.Is(err, ErrTooMany) {
		t.Errorf("upload after %d upload-delete rounds: err = %v, want ErrTooMany", DailyUploads, err)
	}
}

func TestUnvoteTakesTheVoteBack(t *testing.T) {
	h := newHarness()
	id := h.goLive(t, usn)
	h.svc.Vote(context.Background(), "4MC23AI002", id)
	if removed, err := h.svc.Unvote(context.Background(), "4MC23AI002", id); err != nil || !removed {
		t.Fatalf("Unvote = %v, %v", removed, err)
	}
	if n, _ := h.repo.Get(context.Background(), id); n.Votes != 0 {
		t.Errorf("votes = %d", n.Votes)
	}
}

func TestInitOpensAPendingNote(t *testing.T) {
	h := newHarness()
	r, err := h.svc.Init(context.Background(), usn, in())
	if err != nil || r.NoteID != "n1" || r.Session != "https://upload.example/n1" {
		t.Fatalf("Init = %+v, %v", r, err)
	}
	n := h.repo.notes["n1"]
	if n.Status != "pending" || n.Uploader != usn || n.Dept != "AI" || n.Code != "23AI501" || n.Storage != "gdrive" {
		t.Errorf("pending note = %+v", n)
	}
	want := Upload{NoteID: "n1", MIME: "application/pdf", Size: 2048, Origin: "https://meowance.vercel.app", Code: "23AI501"}
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
	var added []bool
	for _, who := range []string{"4MC23AI002", "4MC23AI002", "4MC23CS003"} {
		a, err := h.svc.Vote(context.Background(), who, id)
		if err != nil {
			t.Fatal(err)
		}
		added = append(added, a)
	}
	if fmt.Sprint(added) != "[true false true]" {
		t.Errorf("Vote reported counted = %v, want [true false true] so the screen can tell a repeat", added)
	}
	got, _ := h.svc.List(context.Background(), "23AI501")
	if got[0].Votes != 2 {
		t.Errorf("votes = %d, want 2 (a repeat vote doesn't count)", got[0].Votes)
	}
	if _, err := h.svc.Vote(context.Background(), usn, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("vote on a missing note: %v", err)
	}
	r, _ := h.svc.Init(context.Background(), usn, in())
	if _, err := h.svc.Vote(context.Background(), usn, r.NoteID); !errors.Is(err, ErrNotFound) {
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

func TestReportIsRetriedWhenTheEmailFails(t *testing.T) {
	h := newHarness()
	id := h.goLive(t, usn)
	sendErr := errors.New("resend is down")
	h.svc.Notify = func(context.Context, ReportInfo) error { return sendErr }
	if err := h.svc.Report(context.Background(), "4MC23AI002", id, "textbook"); !errors.Is(err, sendErr) {
		t.Fatalf("a failed email was hidden: %v", err)
	}
	// the student tries again once the mail works: Max must get it this time
	h.svc.Notify = func(_ context.Context, r ReportInfo) error { h.reported = append(h.reported, r); return nil }
	if err := h.svc.Report(context.Background(), "4MC23AI002", id, "textbook"); err != nil {
		t.Fatal(err)
	}
	if len(h.reported) != 1 {
		t.Errorf("Max was emailed %d times, want exactly 1 (the lost one must be re-sent)", len(h.reported))
	}
}

// strictRepo fails Delete and Reject on a cancelled context, like a real database would.
type strictRepo struct{ Repo }

func (r strictRepo) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.Repo.Delete(ctx, id)
}
func (r strictRepo) Reject(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.Repo.Reject(ctx, id)
}

func TestCleanupStillRunsWhenTheRequestIsCancelled(t *testing.T) {
	t.Run("a failed upload session gives its slot back", func(t *testing.T) {
		h := newHarness()
		h.svc.Repo = strictRepo{h.repo}
		ctx, cancel := context.WithCancel(context.Background())
		h.store.onNew = cancel // the request runs out of time inside the store call
		h.store.newErr = context.Canceled
		if _, err := h.svc.Init(ctx, usn, in()); err == nil {
			t.Fatal("expected an error")
		}
		if len(h.repo.notes) != 0 {
			t.Errorf("the pending note stayed (%d rows): the student lost a daily slot to our failure", len(h.repo.notes))
		}
	})
	t.Run("a rejected upload is still discarded and marked", func(t *testing.T) {
		h := newHarness()
		r, _ := h.svc.Init(context.Background(), usn, in())
		h.svc.Repo = strictRepo{h.repo}
		ctx, cancel := context.WithCancel(context.Background())
		h.store.onFinish = cancel
		h.store.finErr = ErrRejected
		if _, err := h.svc.Commit(ctx, usn, r.NoteID, "k"); !errors.Is(err, ErrRejected) {
			t.Fatalf("err = %v", err)
		}
		if n := h.repo.notes[r.NoteID]; n == nil || n.Status != "rejected" {
			t.Errorf("note = %+v, want it marked rejected so it keeps counting", n)
		}
		if len(h.store.discarded) != 1 {
			t.Errorf("discarded = %v: the stray file would stay in Drive", h.store.discarded)
		}
	})
}

func TestValidCode(t *testing.T) {
	for _, ok := range []string{"23AI501", "23ai501", " 23AI704D ", "23OEEC71"} {
		if !ValidCode(ok) {
			t.Errorf("%q should be a course code", ok)
		}
	}
	for _, bad := range []string{"", "ab", "23 AI501", "23AI501;", "x' or 'a'='a", "1234567890123"} {
		if ValidCode(bad) {
			t.Errorf("%q should not be a course code", bad)
		}
	}
}
