package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/meowance/pkg/notes"
	"github.com/dizziedbliss/meowance/pkg/portal"
)

const (
	origin = "https://meowance.vercel.app"
	usnA   = "4MC23AI001"
	usnB   = "4MC23CS002"
	secret = "0123456789abcdef0123456789abcdef"
)

type notesRig struct {
	t        *testing.T
	h        http.HandlerFunc
	fetch    *fakeFetcher
	store    *notes.MemStore
	srv      *httptest.Server
	signer   *notes.Signer
	reported []notes.ReportInfo
}

func newNotesRig(t *testing.T) *notesRig {
	t.Helper()
	r := &notesRig{t: t, fetch: &fakeFetcher{d: &portal.Snapshot{}}}
	mux := http.NewServeMux()
	r.srv = httptest.NewServer(mux)
	t.Cleanup(r.srv.Close)
	r.store = notes.NewMemStore(r.srv.URL)
	r.store.Routes(mux)
	var err error
	if r.signer, err = notes.NewSigner(secret); err != nil {
		t.Fatal(err)
	}
	svc := &notes.Service{Repo: notes.NewMemRepo(), Store: r.store,
		Notify: func(_ context.Context, ri notes.ReportInfo) error { r.reported = append(r.reported, ri); return nil }}
	deps := &NotesDeps{Svc: svc, Signer: r.signer, Portal: r.fetch, Origins: []string{origin}}
	r.h = notesHandler(func() (*NotesDeps, error) { return deps, nil }, newLimiter(time.Now))
	return r
}

// call sends one request; token and origin are optional.
func (r *notesRig) call(method, target, body, token, org string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("X-Real-IP", "1.2.3.4")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if org != "" {
		req.Header.Set("Origin", org)
	}
	rec := httptest.NewRecorder()
	r.h(rec, req)
	return rec
}

func (r *notesRig) login(usn string) string {
	r.t.Helper()
	rec := r.call("POST", "/api/notes", `{"action":"login","usn":"`+usn+`","dob":"15082004"}`, "", "")
	var out struct{ Token string }
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Token == "" {
		r.t.Fatalf("login = %d %s", rec.Code, rec.Body)
	}
	return out.Token
}

func errCode(rec *httptest.ResponseRecorder) string {
	var out struct{ Error string }
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Error
}

const initBody = `{"action":"init","title":"Unit 3 notes","code":"23ai501","mime":"application/pdf","size":6}`

// upload runs init, sends the file to the store like the browser would, and returns the note id and key.
func (r *notesRig) upload(token string) (id, key string) {
	r.t.Helper()
	rec := r.call("POST", "/api/notes", initBody, token, origin)
	var in struct{ NoteID, Session string }
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &in) != nil || in.Session == "" {
		r.t.Fatalf("init = %d %s", rec.Code, rec.Body)
	}
	req, _ := http.NewRequest("PUT", in.Session, strings.NewReader("%PDF-1"))
	req.Header.Set("Content-Type", "application/pdf")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer resp.Body.Close()
	var up struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&up)
	return in.NoteID, up.ID
}

func TestNotesLogin(t *testing.T) {
	r := newNotesRig(t)
	tok := r.login(" 4mc23ai001 ")
	if usn, ok := r.signer.Verify(tok); !ok || usn != usnA {
		t.Errorf("token verifies as %q, %v", usn, ok)
	}
	if r.fetch.got.usn != usnA || r.fetch.got.dob.Format("02012006") != "15082004" {
		t.Errorf("the portal was asked about %q / %v", r.fetch.got.usn, r.fetch.got.dob)
	}

	cases := map[string]struct {
		body   string
		fetch  error
		status int
		code   string
	}{
		"wrong birthday": {`{"action":"login","usn":"4MC23AI001","dob":"15082004"}`, portal.ErrBadLogin, 401, "bad_login"},
		"portal down":    {`{"action":"login","usn":"4MC23AI001","dob":"15082004"}`, portal.ErrPortalDown, 502, "portal_down"},
		"portal blocked": {`{"action":"login","usn":"4MC23AI001","dob":"15082004"}`, portal.ErrForbidden, 502, "portal_blocked"},
		"bad usn":        {`{"action":"login","usn":"nope","dob":"15082004"}`, nil, 400, "bad_input"},
		"bad dob":        {`{"action":"login","usn":"4MC23AI001","dob":"31022004"}`, nil, 400, "bad_input"},
	}
	for name, c := range cases {
		r := newNotesRig(t)
		if c.fetch != nil {
			r.fetch.d, r.fetch.err = nil, c.fetch
		}
		rec := r.call("POST", "/api/notes", c.body, "", "")
		if rec.Code != c.status || errCode(rec) != c.code || strings.Contains(rec.Body.String(), "token") {
			t.Errorf("%s: %d %s, want %d %s and no token", name, rec.Code, rec.Body, c.status, c.code)
		}
	}
}

func TestNotesLoginSharesTheBirthdayGuess(t *testing.T) {
	r := newNotesRig(t)
	r.fetch.d, r.fetch.err = nil, portal.ErrBadLogin
	for i := 0; i < 5; i++ {
		if rec := r.call("POST", "/api/notes", `{"action":"login","usn":"4MC23AI001","dob":"15082004"}`, "", ""); rec.Code != 401 {
			t.Fatalf("try %d = %d", i+1, rec.Code)
		}
	}
	rec := r.call("POST", "/api/notes", `{"action":"login","usn":"4MC23AI001","dob":"15082004"}`, "", "")
	if rec.Code != 429 || errCode(rec) != "too_many_tries" {
		t.Errorf("6th wrong birthday = %d %s, want 429 too_many_tries", rec.Code, rec.Body)
	}
}

func TestNotesActionsNeedAValidToken(t *testing.T) {
	r := newNotesRig(t)
	for _, tok := range []string{"", "garbage", "a.b"} {
		for _, body := range []string{initBody, `{"action":"vote","noteId":"x"}`, `{"action":"commit","noteId":"x","key":"y"}`, `{"action":"report","noteId":"x","reason":"z"}`} {
			rec := r.call("POST", "/api/notes", body, tok, origin)
			if rec.Code != 401 || errCode(rec) != "bad_token" {
				t.Errorf("token %q: %d %s, want 401 bad_token", tok, rec.Code, rec.Body)
			}
		}
	}
}

func TestNotesFullFlow(t *testing.T) {
	r := newNotesRig(t)
	tok := r.login(usnA)
	id, key := r.upload(tok)

	rec := r.call("POST", "/api/notes", `{"action":"commit","noteId":"`+id+`","key":"`+key+`"}`, tok, "")
	if rec.Code != 200 {
		t.Fatalf("commit = %d %s", rec.Code, rec.Body)
	}

	rec = r.call("GET", "/api/notes?code=23ai501", "", "", "")
	var list struct {
		Notes []struct{ ID, Title, Label string }
	}
	json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || len(list.Notes) != 1 || list.Notes[0].ID != id || list.Notes[0].Title != "Unit 3 notes" || list.Notes[0].Label == "" {
		t.Fatalf("list = %d %s", rec.Code, rec.Body)
	}

	rec = r.call("GET", "/api/notes?file="+id, "", "", "")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != r.store.Link(key) {
		t.Errorf("file = %d → %q, want a redirect to the store's link", rec.Code, rec.Header().Get("Location"))
	}

	tokB := r.login(usnB)
	for i := 0; i < 2; i++ { // a repeat vote is fine, it just doesn't count twice
		if rec := r.call("POST", "/api/notes", `{"action":"vote","noteId":"`+id+`"}`, tokB, ""); rec.Code != 200 {
			t.Fatalf("vote = %d %s", rec.Code, rec.Body)
		}
	}
	rec = r.call("GET", "/api/notes?code=23AI501", "", "", "")
	if !strings.Contains(rec.Body.String(), `"votes":1`) {
		t.Errorf("votes after a repeat vote: %s", rec.Body)
	}
	if rec := r.call("POST", "/api/notes", `{"action":"report","noteId":"`+id+`","reason":"textbook"}`, tokB, ""); rec.Code != 200 {
		t.Fatalf("report = %d %s", rec.Code, rec.Body)
	}
	if len(r.reported) != 1 || r.reported[0].Reporter != usnB || r.reported[0].Reason != "textbook" {
		t.Errorf("reported = %+v", r.reported)
	}
}

func TestNotesNeverLeakAUSN(t *testing.T) {
	r := newNotesRig(t)
	tok := r.login(usnA)
	id, key := r.upload(tok)
	commit := r.call("POST", "/api/notes", `{"action":"commit","noteId":"`+id+`","key":"`+key+`"}`, tok, "")
	list := r.call("GET", "/api/notes?code=23AI501", "", "", "")
	for name, rec := range map[string]*httptest.ResponseRecorder{"commit": commit, "list": list} {
		if strings.Contains(strings.ToUpper(rec.Body.String()), "4MC") {
			t.Errorf("%s response leaks a USN: %s", name, rec.Body)
		}
	}
}

func TestNotesInitChecksOriginAndLimit(t *testing.T) {
	r := newNotesRig(t)
	tok := r.login(usnA)
	if rec := r.call("POST", "/api/notes", initBody, tok, "https://evil.example"); rec.Code != 403 || errCode(rec) != "forbidden" {
		t.Errorf("foreign origin = %d %s, want 403 forbidden", rec.Code, rec.Body)
	}
	if rec := r.call("POST", "/api/notes", initBody, tok, ""); rec.Code != 400 || errCode(rec) != "bad_input" {
		t.Errorf("no origin = %d %s, want 400 bad_input", rec.Code, rec.Body)
	}
	for i := 0; i < notes.DailyUploads; i++ {
		if rec := r.call("POST", "/api/notes", initBody, tok, origin); rec.Code != 200 {
			t.Fatalf("init %d = %d %s", i+1, rec.Code, rec.Body)
		}
	}
	if rec := r.call("POST", "/api/notes", initBody, tok, origin); rec.Code != 429 || errCode(rec) != "too_many_uploads" {
		t.Errorf("over the daily limit = %d %s, want 429 too_many_uploads", rec.Code, rec.Body)
	}
	bad := `{"action":"init","title":"","code":"23ai501","mime":"application/pdf","size":6}`
	if rec := r.call("POST", "/api/notes", bad, r.login(usnB), origin); rec.Code != 400 || errCode(rec) != "bad_input" {
		t.Errorf("empty title = %d %s, want 400 bad_input", rec.Code, rec.Body)
	}
}

func TestNotesCommitErrors(t *testing.T) {
	r := newNotesRig(t)
	tokA, tokB := r.login(usnA), r.login(usnB)
	id, key := r.upload(tokA)
	commit := func(tok, id, key string) *httptest.ResponseRecorder {
		return r.call("POST", "/api/notes", `{"action":"commit","noteId":"`+id+`","key":"`+key+`"}`, tok, "")
	}
	if rec := commit(tokB, id, key); rec.Code != 403 || errCode(rec) != "forbidden" {
		t.Errorf("someone else's upload = %d %s, want 403 forbidden", rec.Code, rec.Body)
	}
	if rec := commit(tokA, "nope", key); rec.Code != 404 || errCode(rec) != "not_found" {
		t.Errorf("unknown note = %d %s, want 404 not_found", rec.Code, rec.Body)
	}
	if rec := commit(tokA, id, "dev-someoneelses"); rec.Code != 422 || errCode(rec) != "rejected" {
		t.Errorf("wrong key = %d %s, want 422 rejected", rec.Code, rec.Body)
	}
}

func TestNotesReadErrors(t *testing.T) {
	r := newNotesRig(t)
	if rec := r.call("GET", "/api/notes?code=bad%20code", "", "", ""); rec.Code != 400 {
		t.Errorf("bad course code = %d, want 400", rec.Code)
	}
	rec := r.call("GET", "/api/notes?code=23AI999", "", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"notes":[]`) {
		t.Errorf("empty course = %d %s, want an empty list, not null", rec.Code, rec.Body)
	}
	if rec := r.call("GET", "/api/notes?file=nope", "", "", ""); rec.Code != 404 {
		t.Errorf("unknown file = %d, want 404", rec.Code)
	}
	if rec := r.call("GET", "/api/notes", "", "", ""); rec.Code != 400 {
		t.Errorf("GET with nothing asked = %d, want 400", rec.Code)
	}
	if rec := r.call("PUT", "/api/notes", "{}", "", ""); rec.Code != 405 {
		t.Errorf("PUT = %d, want 405", rec.Code)
	}
	if rec := r.call("POST", "/api/notes", `{"action":"explode"}`, "", ""); rec.Code != 400 {
		t.Errorf("unknown action = %d, want 400", rec.Code)
	}
	if rec := r.call("GET", "/api/notes?code=23AI501", "", "", ""); rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
}

func TestNotesOffWhenNotConfigured(t *testing.T) {
	h := notesHandler(func() (*NotesDeps, error) { return nil, errors.New("no drive") }, newLimiter(time.Now))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/api/notes?code=23AI501", nil))
	if rec.Code != 503 || errCode(rec) != "notes_off" {
		t.Errorf("unconfigured = %d %s, want 503 notes_off", rec.Code, rec.Body)
	}
}

func TestReportNotifierEmailsMaxTheDetails(t *testing.T) {
	m := &fakeMailer{}
	err := ReportNotifier(m)(context.Background(), notes.ReportInfo{NoteID: "abc", Title: "Unit 3", Code: "23AI501", Reason: "textbook", Reporter: usnB})
	if err != nil || len(m.sent) != 1 {
		t.Fatalf("sent %v, %v", m.sent, err)
	}
	for _, want := range []string{usnB, "abc", "Unit 3", "23AI501", "textbook"} {
		if !strings.Contains(m.sent[0], want) {
			t.Errorf("report email %q is missing %q", m.sent[0], want)
		}
	}
	if ReportNotifier(nil) != nil {
		t.Error("with no mailer there is nothing to notify")
	}
}
