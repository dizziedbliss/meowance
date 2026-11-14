package notes

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMemStoreRoundTrip(t *testing.T) {
	m := NewMemStore("http://localhost:8787")
	mux := http.NewServeMux()
	m.Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u := Upload{NoteID: "n1", MIME: "application/pdf", Size: 5, Origin: "o", Code: "23AI501"}
	session, err := m.NewUpload(context.Background(), u)
	if err != nil || session != "http://localhost:8787/api/dev-upload/n1" {
		t.Fatalf("NewUpload = %q, %v", session, err)
	}
	put := func(body string) string {
		req, _ := http.NewRequest("PUT", srv.URL+"/api/dev-upload/n1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/pdf")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	if _, err := m.Finish(context.Background(), u, "dev-n1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Finish before any upload = %v, want ErrNotFound", err)
	}
	if got := put("%PDF-"); !strings.Contains(got, `"id":"dev-n1"`) {
		t.Errorf("upload reply = %s", got)
	}
	st, err := m.Finish(context.Background(), u, "dev-n1")
	if err != nil || st.Size != 5 {
		t.Fatalf("Finish = %+v, %v", st, err)
	}
	resp, _ := http.Get(srv.URL + "/api/dev-file/dev-n1")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "%PDF-" || resp.Header.Get("Content-Type") != "application/pdf" {
		t.Errorf("served %q as %q", body, resp.Header.Get("Content-Type"))
	}
	if _, err := m.Finish(context.Background(), u, "dev-other"); !errors.Is(err, ErrRejected) {
		t.Errorf("Finish with someone else's key = %v, want ErrRejected", err)
	}
	put("too long for the announced size")
	if _, err := m.Finish(context.Background(), u, "dev-n1"); !errors.Is(err, ErrRejected) {
		t.Errorf("Finish with the wrong size = %v, want ErrRejected", err)
	}
	if err := m.Discard(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Finish(context.Background(), u, "dev-n1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Finish after Discard = %v, want ErrNotFound", err)
	}
}
