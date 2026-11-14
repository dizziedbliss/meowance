package notes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// MemStore is a Store that keeps files in memory, for the local dev server and
// tests. Routes serves the "browser uploads straight to the store" PUT and the file
// links, so the whole notes flow can be tried offline. Never production.
type MemStore struct {
	base  string // where the dev server listens, e.g. http://localhost:8787
	mu    sync.Mutex
	files map[string]memFile // keyed by note id
}

type memFile struct {
	data []byte
	mime string
}

func NewMemStore(base string) *MemStore {
	return &MemStore{base: strings.TrimRight(base, "/"), files: map[string]memFile{}}
}

func (m *MemStore) NewUpload(_ context.Context, u Upload) (string, error) {
	if err := u.check(); err != nil {
		return "", err
	}
	return m.base + "/api/dev-upload/" + u.NoteID, nil
}

func (m *MemStore) Finish(_ context.Context, u Upload, key string) (Stored, error) {
	if err := u.check(); err != nil {
		return Stored{}, err
	}
	if key != "dev-"+u.NoteID {
		return Stored{}, ErrRejected
	}
	m.mu.Lock()
	f, ok := m.files[u.NoteID]
	m.mu.Unlock()
	if !ok {
		return Stored{}, ErrNotFound
	}
	if int64(len(f.data)) != u.Size || f.mime != u.MIME {
		return Stored{}, ErrRejected
	}
	return Stored{Key: key, Size: u.Size, MIME: u.MIME}, nil
}

func (m *MemStore) Link(key string) string { return m.base + "/api/dev-file/" + key }

func (m *MemStore) Discard(_ context.Context, u Upload) error {
	m.mu.Lock()
	delete(m.files, u.NoteID)
	m.mu.Unlock()
	return nil
}

func (m *MemStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.files, strings.TrimPrefix(key, "dev-"))
	m.mu.Unlock()
	return nil
}

// Routes registers the dev-only upload and download endpoints.
func (m *MemStore) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/dev-upload/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/dev-upload/")
		if r.Method != http.MethodPut || !idPattern.MatchString(id) {
			http.Error(w, "bad upload", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, MaxFileSize+1))
		if err != nil || len(data) > MaxFileSize {
			http.Error(w, "too big", http.StatusRequestEntityTooLarge)
			return
		}
		m.mu.Lock()
		m.files[id] = memFile{data: data, mime: r.Header.Get("Content-Type")}
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "dev-" + id})
	})
	mux.HandleFunc("/api/dev-file/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/api/dev-file/")
		m.mu.Lock()
		f, ok := m.files[strings.TrimPrefix(key, "dev-")]
		m.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", f.mime)
		w.Write(f.data)
	})
}
