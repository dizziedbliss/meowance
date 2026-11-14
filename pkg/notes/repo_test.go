package notes

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var base = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func note(id, usn, code string, at time.Time) Note {
	return Note{ID: id, Code: code, Dept: "AI", Title: "t " + id, Storage: "gdrive",
		MIME: "application/pdf", Size: 10, Uploader: usn, Status: "pending", Created: at}
}

// repoContract is what every Repo must do: the in-memory one always, Postgres when it is available.
func repoContract(t *testing.T, mk func(t *testing.T) Repo) {
	ctx := context.Background()
	day := base.Add(-24 * time.Hour)

	t.Run("limit counts a student's notes in the window", func(t *testing.T) {
		r := mk(t)
		for i := 0; i < 5; i++ {
			ok, err := r.CreateWithin(ctx, note(fmt.Sprint("a", i), "U1", "C1", base), day, 5)
			if err != nil || !ok {
				t.Fatalf("note %d: %v, %v", i, ok, err)
			}
		}
		if ok, _ := r.CreateWithin(ctx, note("a5", "U1", "C1", base), day, 5); ok {
			t.Error("a 6th note got in")
		}
		if ok, _ := r.CreateWithin(ctx, note("b0", "U2", "C1", base), day, 5); !ok {
			t.Error("another student was blocked")
		}
		if ok, _ := r.CreateWithin(ctx, note("a6", "U1", "C1", base.Add(25*time.Hour)), base.Add(time.Hour), 5); !ok {
			t.Error("notes older than the window still counted")
		}
	})

	t.Run("limit holds under parallel requests", func(t *testing.T) {
		r := mk(t)
		var wg sync.WaitGroup
		var mu sync.Mutex
		got := 0
		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if ok, err := r.CreateWithin(ctx, note(fmt.Sprint("p", i), "U1", "C1", base), day, 5); err == nil && ok {
					mu.Lock()
					got++
					mu.Unlock()
				}
			}(i)
		}
		wg.Wait()
		if got != 5 {
			t.Errorf("%d of 30 parallel creates got in, want exactly 5", got)
		}
	})

	t.Run("a rejected note keeps counting", func(t *testing.T) {
		r := mk(t)
		for i := 0; i < 5; i++ {
			id := fmt.Sprint("r", i)
			r.CreateWithin(ctx, note(id, "U1", "C1", base), day, 5)
			if err := r.Reject(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
		if ok, _ := r.CreateWithin(ctx, note("r5", "U1", "C1", base), day, 5); ok {
			t.Error("rejected notes stopped counting")
		}
		if err := r.Reject(ctx, "missing"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Reject(missing) = %v, want ErrNotFound", err)
		}
	})

	t.Run("get and set live", func(t *testing.T) {
		r := mk(t)
		if _, err := r.Get(ctx, "nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(missing) = %v, want ErrNotFound", err)
		}
		r.CreateWithin(ctx, note("g1", "U1", "C1", base), day, 5)
		n, err := r.Get(ctx, "g1")
		if err != nil || n.Status != "pending" || n.Uploader != "U1" || n.Title != "t g1" || !n.Created.Equal(base) {
			t.Fatalf("Get = %+v, %v", n, err)
		}
		if err := r.SetLive(ctx, "g1", "key1", 99, "image/png"); err != nil {
			t.Fatal(err)
		}
		n, _ = r.Get(ctx, "g1")
		if n.Status != "live" || n.FileKey != "key1" || n.Size != 99 || n.MIME != "image/png" {
			t.Errorf("after SetLive = %+v", n)
		}
		if err := r.SetLive(ctx, "g1", "key2", 1, "x"); !errors.Is(err, ErrNotFound) {
			t.Errorf("SetLive on a live note = %v, want ErrNotFound", err)
		}
		if err := r.SetLive(ctx, "nope", "k", 1, "x"); !errors.Is(err, ErrNotFound) {
			t.Errorf("SetLive(missing) = %v, want ErrNotFound", err)
		}
	})

	t.Run("list is the course's live notes, newest first, with votes", func(t *testing.T) {
		r := mk(t)
		for i, id := range []string{"l1", "l2", "l3"} {
			r.CreateWithin(ctx, note(id, fmt.Sprint("U", i), "C1", base.Add(time.Duration(i)*time.Minute)), day, 5)
		}
		r.CreateWithin(ctx, note("other", "U9", "C2", base), day, 5)
		r.SetLive(ctx, "l1", "k1", 1, "application/pdf")
		r.SetLive(ctx, "l3", "k3", 1, "application/pdf")
		r.SetLive(ctx, "other", "ko", 1, "application/pdf")
		r.Vote(ctx, "l1", "A")
		r.Vote(ctx, "l1", "B")
		got, err := r.ListLive(ctx, "C1")
		if err != nil || len(got) != 2 || got[0].ID != "l3" || got[1].ID != "l1" {
			t.Fatalf("ListLive = %+v, %v (want l3, l1: l2 is pending, other is another course)", got, err)
		}
		if got[1].Votes != 2 || got[0].Votes != 0 {
			t.Errorf("votes = %d, %d", got[0].Votes, got[1].Votes)
		}
	})

	t.Run("one vote and one report per student", func(t *testing.T) {
		r := mk(t)
		r.CreateWithin(ctx, note("v1", "U1", "C1", base), day, 5)
		for i, want := range []bool{true, false} {
			if added, err := r.Vote(ctx, "v1", "A"); err != nil || added != want {
				t.Errorf("vote %d = %v, %v, want %v", i, added, err, want)
			}
			if added, err := r.Report(ctx, "v1", "A", "why"); err != nil || added != want {
				t.Errorf("report %d = %v, %v, want %v", i, added, err, want)
			}
		}
		if added, _ := r.Vote(ctx, "v1", "B"); !added {
			t.Error("a different student's vote was refused")
		}
	})

	t.Run("a report can be taken back so it can be sent again", func(t *testing.T) {
		r := mk(t)
		r.CreateWithin(ctx, note("t1", "U1", "C1", base), day, 5)
		if added, _ := r.Report(ctx, "t1", "A", "why"); !added {
			t.Fatal("first report not added")
		}
		if err := r.RemoveReport(ctx, "t1", "A"); err != nil {
			t.Fatal(err)
		}
		if added, _ := r.Report(ctx, "t1", "A", "why"); !added {
			t.Error("a report that was taken back should count as new")
		}
		if err := r.RemoveReport(ctx, "t1", "nobody"); err != nil {
			t.Errorf("removing a report that isn't there = %v, want nil", err)
		}
	})

	t.Run("delete removes the note and its votes", func(t *testing.T) {
		r := mk(t)
		r.CreateWithin(ctx, note("d1", "U1", "C1", base), day, 5)
		r.Vote(ctx, "d1", "A")
		r.Report(ctx, "d1", "A", "x")
		if err := r.Delete(ctx, "d1"); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Get(ctx, "d1"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get after Delete = %v", err)
		}
		if err := r.Delete(ctx, "d1"); err != nil {
			t.Errorf("deleting twice = %v, want nil", err)
		}
	})
}

func TestMemRepoContract(t *testing.T) {
	repoContract(t, func(*testing.T) Repo { return newMemRepo() })
}

func TestMemRepoSurvivesParallelRequests(t *testing.T) {
	r := newMemRepo()
	ctx := context.Background()
	r.CreateWithin(ctx, note("p1", "U1", "C1", base), base.Add(-time.Hour), 100)
	r.SetLive(ctx, "p1", "k", 1, "application/pdf")
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Vote(ctx, "p1", fmt.Sprint("V", i))
			r.Report(ctx, "p1", fmt.Sprint("V", i), "x")
			r.ListLive(ctx, "C1")
			r.CreateWithin(ctx, note(fmt.Sprint("q", i), fmt.Sprint("U", i), "C1", base), base.Add(-time.Hour), 100)
			r.SetLive(ctx, fmt.Sprint("q", i), "k", 1, "application/pdf")
		}(i)
	}
	wg.Wait() // -race (and a map write crash) would flag any unlocked access
}
