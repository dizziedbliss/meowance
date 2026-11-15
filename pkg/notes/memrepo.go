package notes

import (
	"context"
	"sort"
	"sync"
	"time"
)

// NewMemRepo is a Repo kept in memory: for tests and the local dev server, never production.
func NewMemRepo() Repo { return newMemRepo() }

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
func (r *memRepo) Retire(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.notes[id]
	if !ok || n.Status != "live" {
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
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.notes[id]
	if !ok || n.Status != "pending" {
		return ErrNotFound
	}
	n.Status, n.FileKey, n.Size, n.MIME = "live", key, size, mime
	return nil
}
func (r *memRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.notes, id)
	delete(r.votes, id)
	delete(r.reports, id)
	return nil
}
func (r *memRepo) ListLive(_ context.Context, code string) ([]Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.votes[id] == nil {
		r.votes[id] = map[string]bool{}
	}
	if r.votes[id][usn] {
		return false, nil
	}
	r.votes[id][usn] = true
	return true, nil
}
func (r *memRepo) Unvote(_ context.Context, id, usn string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	had := r.votes[id][usn]
	delete(r.votes[id], usn)
	return had, nil
}
func (r *memRepo) Mine(_ context.Context, code, usn string) (mine, voted []string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, n := range r.notes {
		if n.Status != "live" || n.Code != code {
			continue
		}
		if n.Uploader == usn {
			mine = append(mine, id)
		}
		if r.votes[id][usn] {
			voted = append(voted, id)
		}
	}
	return mine, voted, nil
}
func (r *memRepo) RemoveReport(_ context.Context, id, usn string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.reports[id], usn)
	return nil
}
func (r *memRepo) Report(_ context.Context, id, usn, reason string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reports[id] == nil {
		r.reports[id] = map[string]string{}
	}
	if _, dup := r.reports[id][usn]; dup {
		return false, nil
	}
	r.reports[id][usn] = reason
	return true, nil
}
