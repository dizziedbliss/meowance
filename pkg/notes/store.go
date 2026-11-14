package notes

import (
	"context"
	"errors"
	"regexp"
)

// Store is where uploaded note files live. Notes record which Store holds them
// and a key; the app's own links redirect to Link(key), so moving files to a new
// Store later (or a new account) never breaks a link anyone has shared.
type Store interface {
	// NewUpload opens a session the browser can send the file to directly, so the
	// file never passes through our function (Vercel caps request bodies at 4.5 MB).
	NewUpload(ctx context.Context, u Upload) (session string, err error)
	// Finish checks that what landed is the upload we announced, then makes it
	// viewable by link.
	Finish(ctx context.Context, u Upload, key string) (Stored, error)
	// Discard removes whatever the store holds under this upload's own file name (the
	// note id), after a failed check. It never touches a file named by the client.
	Discard(ctx context.Context, u Upload) error
	// Link is where a student is sent to open or download the file.
	Link(key string) string
	// Delete removes the file; one that is already gone is not an error.
	Delete(ctx context.Context, key string) error
}

// Upload is one announced upload. NoteID is made by the server, never the client.
type Upload struct {
	NoteID string // names the stored file; never a student's filename
	MIME   string
	Size   int64
	Origin string // the page's origin, so the browser is allowed to send to the session
	Dept   string // 2 letters from the USN, e.g. "AI"
	Sem    string
	Code   string // course code, e.g. "23AI501"
}

// Stored is a file that has landed and been checked.
type Stored struct {
	Key  string
	Size int64
	MIME string
}

// MaxFileSize is the biggest note file accepted.
const MaxFileSize = 25 << 20

var (
	ErrBadUpload = errors.New("notes: upload not allowed")
	ErrRejected  = errors.New("notes: the stored file isn't the announced upload")
	ErrNotFound  = errors.New("notes: file not found")
)

var (
	allowedMIME = map[string]string{"application/pdf": "pdf", "image/png": "png", "image/jpeg": "jpg"}
	idPattern   = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)    // note ids and folder names
	keyPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`) // a Drive file id
)

// check reports whether the upload is something we'd store. Folder names and the
// note id are restricted to letters and digits, so they can't change a Drive query.
func (u Upload) check() error {
	switch {
	case allowedMIME[u.MIME] == "":
		return errBad("file type")
	case u.Size <= 0 || u.Size > MaxFileSize:
		return errBad("file size")
	case !idPattern.MatchString(u.NoteID):
		return errBad("note id")
	case !idPattern.MatchString(u.Dept), !idPattern.MatchString(u.Sem), !idPattern.MatchString(u.Code):
		return errBad("course")
	}
	return nil
}

func (u Upload) fileName() string { return u.NoteID + "." + allowedMIME[u.MIME] }

func errBad(what string) error { return &badUpload{what} }

type badUpload struct{ what string }

func (e *badUpload) Error() string        { return "notes: upload not allowed: " + e.what }
func (e *badUpload) Is(target error) bool { return target == ErrBadUpload }
