package notes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDrive is just enough of Google's Drive and OAuth endpoints to test the store.
type fakeDrive struct {
	mu          sync.Mutex
	files       map[string]*fakeFile // id -> file
	next        int
	tokenCalls  int
	creates     int // folder creates
	lastInit    http.Header
	lastInitMet map[string]any
	shared      []string
	deleted     []string
	failNext    bool // make the next API call 500
}

type fakeFile struct {
	ID, Name, MIME, Parent string
	Size                   string
}

func newFake(t *testing.T) (*fakeDrive, *Drive) {
	t.Helper()
	f := &fakeDrive{files: map[string]*fakeFile{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	d := NewDrive(DriveConfig{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	d.tokenURL, d.apiBase, d.uploadBase = srv.URL+"/token", srv.URL, srv.URL
	return f, d
}

var qName = regexp.MustCompile(`name='([^']*)'`)
var qParent = regexp.MustCompile(`'([^']*)' in parents`)

func (f *fakeDrive) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/token" {
		f.tokenCalls++
		json.NewEncoder(w).Encode(map[string]any{"access_token": fmt.Sprintf("at%d", f.tokenCalls), "expires_in": 3600})
		return
	}
	if r.Header.Get("Authorization") == "" {
		http.Error(w, "no token", 401)
		return
	}
	if f.failNext {
		f.failNext = false
		http.Error(w, "boom", 500)
		return
	}
	p := r.URL.Path
	switch {
	case r.Method == "GET" && p == "/drive/v3/files": // folder lookup
		q := r.URL.Query().Get("q")
		name, parent := qName.FindStringSubmatch(q)[1], qParent.FindStringSubmatch(q)[1]
		wantFolder := strings.Contains(q, folderMIME)
		out := []map[string]string{}
		for _, fl := range f.files {
			if (fl.MIME == folderMIME) == wantFolder && fl.Name == name && fl.Parent == parent {
				out = append(out, map[string]string{"id": fl.ID})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"files": out})
	case r.Method == "POST" && p == "/drive/v3/files": // folder create
		var m struct {
			Name    string   `json:"name"`
			MIME    string   `json:"mimeType"`
			Parents []string `json:"parents"`
		}
		json.NewDecoder(r.Body).Decode(&m)
		parent := "root"
		if len(m.Parents) == 1 {
			parent = m.Parents[0]
		}
		f.next++
		f.creates++
		id := fmt.Sprintf("folder%d", f.next)
		f.files[id] = &fakeFile{ID: id, Name: m.Name, MIME: m.MIME, Parent: parent}
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	case r.Method == "POST" && p == "/upload/drive/v3/files": // resumable session
		f.lastInit = r.Header.Clone()
		json.NewDecoder(r.Body).Decode(&f.lastInitMet)
		w.Header().Set("Location", "https://upload.example/session?upload_id=abc")
	case strings.HasSuffix(p, "/permissions") && r.Method == "POST":
		f.shared = append(f.shared, strings.TrimSuffix(strings.TrimPrefix(p, "/drive/v3/files/"), "/permissions"))
	case strings.HasPrefix(p, "/drive/v3/files/") && r.Method == "GET":
		fl := f.files[strings.TrimPrefix(p, "/drive/v3/files/")]
		if fl == nil {
			http.Error(w, "nope", 404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": fl.ID, "name": fl.Name, "mimeType": fl.MIME, "size": fl.Size, "parents": []string{fl.Parent}})
	case strings.HasPrefix(p, "/drive/v3/files/") && r.Method == "DELETE":
		id := strings.TrimPrefix(p, "/drive/v3/files/")
		if f.files[id] == nil {
			http.Error(w, "nope", 404)
			return
		}
		f.deleted = append(f.deleted, id)
		delete(f.files, id)
		w.WriteHeader(204)
	default:
		http.Error(w, "unexpected "+r.Method+" "+p, 400)
	}
}

func up() Upload {
	return Upload{NoteID: "n1a2b3", MIME: "application/pdf", Size: 1234, Origin: "https://meowance.vercel.app", Dept: "AI", Sem: "5", Code: "23AI501"}
}

// land puts a file where the browser upload would have, in the folder the store made.
func (f *fakeDrive) land(t *testing.T, d *Drive, u Upload, name, mime string, size int64) string {
	t.Helper()
	parent, err := d.folder(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	id := fmt.Sprintf("file%d", f.next)
	f.files[id] = &fakeFile{ID: id, Name: name, MIME: mime, Parent: parent, Size: fmt.Sprint(size)}
	return id
}

func TestNewUploadMakesTheFoldersOnceAndOpensASession(t *testing.T) {
	f, d := newFake(t)
	uri, err := d.NewUpload(context.Background(), up())
	if err != nil || uri != "https://upload.example/session?upload_id=abc" {
		t.Fatalf("NewUpload = %q, %v", uri, err)
	}
	if f.creates != 5 { // meowance / notes / AI / 5 / 23AI501
		t.Errorf("made %d folders, want 5", f.creates)
	}
	if got := f.lastInit.Get("Origin"); got != "https://meowance.vercel.app" {
		t.Errorf("Origin = %q: the browser could not PUT to the session", got)
	}
	if f.lastInit.Get("X-Upload-Content-Type") != "application/pdf" || f.lastInit.Get("X-Upload-Content-Length") != "1234" {
		t.Errorf("upload headers wrong: %v", f.lastInit)
	}
	if f.lastInitMet["name"] != "n1a2b3.pdf" {
		t.Errorf("file name = %v: names must be the note id, never a student's", f.lastInitMet["name"])
	}
	// a second note in the same course reuses every folder
	u2 := up()
	u2.NoteID = "n9"
	if _, err := d.NewUpload(context.Background(), u2); err != nil {
		t.Fatal(err)
	}
	if f.creates != 5 {
		t.Errorf("second upload made folders again: %d", f.creates)
	}
}

func TestFindsFoldersThatAlreadyExistInDrive(t *testing.T) {
	f, d := newFake(t)
	if _, err := d.NewUpload(context.Background(), up()); err != nil {
		t.Fatal(err)
	}
	// a fresh store (a cold serverless instance) must find them, not remake them
	d2 := NewDrive(DriveConfig{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	d2.tokenURL, d2.apiBase, d2.uploadBase = d.tokenURL, d.apiBase, d.uploadBase
	if _, err := d2.NewUpload(context.Background(), up()); err != nil {
		t.Fatal(err)
	}
	if f.creates != 5 {
		t.Errorf("cold instance made %d folders, want to reuse the 5", f.creates)
	}
}

func TestAccessTokenIsCachedThenRefreshed(t *testing.T) {
	f, d := newFake(t)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	d.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if _, err := d.NewUpload(context.Background(), up()); err != nil {
			t.Fatal(err)
		}
	}
	if f.tokenCalls != 1 {
		t.Errorf("asked for %d tokens, want 1 while it lasts", f.tokenCalls)
	}
	now = now.Add(2 * time.Hour)
	if _, err := d.NewUpload(context.Background(), up()); err != nil {
		t.Fatal(err)
	}
	if f.tokenCalls != 2 {
		t.Errorf("token calls = %d, want a refresh after expiry", f.tokenCalls)
	}
}

func TestNewUploadRefusesBadUploadsBeforeCallingDrive(t *testing.T) {
	f, d := newFake(t)
	bad := map[string]func(*Upload){
		"big":         func(u *Upload) { u.Size = MaxFileSize + 1 },
		"empty":       func(u *Upload) { u.Size = 0 },
		"mime":        func(u *Upload) { u.MIME = "application/x-msdownload" },
		"note id":     func(u *Upload) { u.NoteID = "../x" },
		"folder name": func(u *Upload) { u.Code = "x' or 'a'='a" },
		"empty dept":  func(u *Upload) { u.Dept = "" },
		"origin":      func(u *Upload) { u.Origin = "" },
	}
	for name, mut := range bad {
		u := up()
		mut(&u)
		if _, err := d.NewUpload(context.Background(), u); !errors.Is(err, ErrBadUpload) {
			t.Errorf("%s: err = %v, want ErrBadUpload", name, err)
		}
	}
	if f.tokenCalls != 0 || f.creates != 0 {
		t.Errorf("Drive was contacted (%d tokens, %d folders) for rejected uploads", f.tokenCalls, f.creates)
	}
}

func TestFinishSharesAGenuineUpload(t *testing.T) {
	f, d := newFake(t)
	id := f.land(t, d, up(), "n1a2b3.pdf", "application/pdf", 1234)
	got, err := d.Finish(context.Background(), up(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got != (Stored{Key: id, Size: 1234, MIME: "application/pdf"}) {
		t.Errorf("Finish = %+v", got)
	}
	if len(f.shared) != 1 || f.shared[0] != id {
		t.Errorf("shared = %v, want the file made link-viewable", f.shared)
	}
}

func TestFinishRejectsFilesThatAreNotTheUpload(t *testing.T) {
	f, d := newFake(t)
	cases := map[string]string{
		"someone else's file name": f.land(t, d, up(), "other.pdf", "application/pdf", 1234),
		"wrong type":               f.land(t, d, up(), "n1a2b3.pdf", "text/html", 1234),
		"bigger than announced":    f.land(t, d, up(), "n1a2b3.pdf", "application/pdf", 9999),
		"over the cap":             f.land(t, d, Upload{NoteID: "x", MIME: "application/pdf", Size: 1, Origin: "o", Dept: "AI", Sem: "5", Code: "23AI501"}, "n1a2b3.pdf", "application/pdf", MaxFileSize+1),
	}
	// a file with the right name but in some other folder
	other := up()
	other.Code = "23AI502"
	cases["right name, wrong folder"] = f.land(t, d, other, "n1a2b3.pdf", "application/pdf", 1234)
	for name, id := range cases {
		if _, err := d.Finish(context.Background(), up(), id); !errors.Is(err, ErrRejected) {
			t.Errorf("%s: err = %v, want ErrRejected", name, err)
		}
	}
	if len(f.shared) != 0 {
		t.Errorf("shared %v: a rejected file must never be made public", f.shared)
	}
}

func TestFinishMissingFileIsNotFound(t *testing.T) {
	_, d := newFake(t)
	if _, err := d.Finish(context.Background(), up(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemovesAndForgivesAlreadyGone(t *testing.T) {
	f, d := newFake(t)
	id := f.land(t, d, up(), "n1a2b3.pdf", "application/pdf", 1234)
	if err := d.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 {
		t.Errorf("deleted = %v", f.deleted)
	}
	if err := d.Delete(context.Background(), id); err != nil {
		t.Errorf("deleting twice should be fine, got %v", err)
	}
}

func TestDiscardRemovesTheStrayFileByItsNoteName(t *testing.T) {
	f, d := newFake(t)
	mine := f.land(t, d, up(), "n1a2b3.pdf", "application/pdf", 1234)
	theirs := f.land(t, d, up(), "other.pdf", "application/pdf", 1234) // another note in the same folder
	if err := d.Discard(context.Background(), up()); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 || f.deleted[0] != mine {
		t.Errorf("deleted = %v, want only %s (never %s)", f.deleted, mine, theirs)
	}
	if err := d.Discard(context.Background(), up()); err != nil {
		t.Errorf("nothing left to discard should be fine: %v", err)
	}
}

func TestDriveErrorsComeBackAsErrors(t *testing.T) {
	f, d := newFake(t)
	if _, err := d.NewUpload(context.Background(), up()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.failNext = true
	f.mu.Unlock()
	u := up()
	u.Code = "23AI999" // new folder, so it has to call Drive
	if _, err := d.NewUpload(context.Background(), u); err == nil {
		t.Error("a 500 from Drive was swallowed")
	}
}

func TestLinks(t *testing.T) {
	_, d := newFake(t)
	if got, want := d.Link("abc123"), "https://drive.google.com/uc?id=abc123&export=download"; got != want {
		t.Errorf("Link = %q, want %q", got, want)
	}
	if strings.Contains(d.Link("a b&c"), "a b&c") {
		t.Error("the key must be escaped in the link")
	}
}
