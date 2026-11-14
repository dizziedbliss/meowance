package notes

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestDriveLive talks to the real Google Drive. It is skipped unless NOTES_LIVE=1 and
// the GOOGLE_* env vars are set, and it needs the refresh token of the notes account:
//
//	NOTES_LIVE=1 GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=... GOOGLE_REFRESH_TOKEN=... go test ./pkg/notes -run Live -v
//
// It uploads a tiny PDF into meowance/notes/LIVETEST, checks the public link,
// then deletes the file (the empty folders stay).
func TestDriveLive(t *testing.T) {
	d := DriveFromEnv()
	if os.Getenv("NOTES_LIVE") != "1" || d == nil {
		t.Skip("set NOTES_LIVE=1 and the GOOGLE_* env vars to run against real Drive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	b := make([]byte, 6)
	rand.Read(b)
	pdf := []byte("%PDF-1.4\n% meowance live test " + hex.EncodeToString(b) + "\n%%EOF\n")
	u := Upload{NoteID: "live" + hex.EncodeToString(b), MIME: "application/pdf", Size: int64(len(pdf)),
		Origin: "https://meowance.vercel.app", Code: "LIVETEST"}

	session, err := d.NewUpload(ctx, u)
	if err != nil {
		t.Fatal("NewUpload:", err)
	}
	req, _ := http.NewRequestWithContext(ctx, "PUT", session, bytes.NewReader(pdf))
	req.Header.Set("Content-Type", u.MIME)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("upload:", err)
	}
	var up struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&up)
	resp.Body.Close()
	if resp.StatusCode != 200 || up.ID == "" {
		t.Fatalf("upload answered %d, id %q", resp.StatusCode, up.ID)
	}
	t.Cleanup(func() { d.Delete(context.Background(), up.ID) })

	got, err := d.Finish(ctx, u, up.ID)
	if err != nil || got.Size != u.Size {
		t.Fatalf("Finish = %+v, %v", got, err)
	}

	// anyone, not logged in, can fetch the file from the link
	lr, err := http.Get(d.Link(up.ID))
	if err != nil {
		t.Fatal("link:", err)
	}
	body, _ := io.ReadAll(io.LimitReader(lr.Body, 1<<20))
	lr.Body.Close()
	if lr.StatusCode != 200 || !bytes.Equal(body, pdf) {
		t.Fatalf("anonymous link gave %d and %d bytes, want the file back", lr.StatusCode, len(body))
	}

	if err := d.Delete(ctx, up.ID); err != nil {
		t.Fatal("Delete:", err)
	}
	if _, err := d.Finish(ctx, u, up.ID); !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrRejected) {
		t.Errorf("after delete, Finish = %v, want not found", err)
	}
}
