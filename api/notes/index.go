// Package handler is the Vercel Go function behind /api/notes (notes sharing).
// The logic lives in pkg/api and pkg/notes so the dev server runs the same code.
package handler

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dizziedbliss/meowance/pkg/api"
	"github.com/dizziedbliss/meowance/pkg/notes"
	"github.com/dizziedbliss/meowance/pkg/portal"
)

var (
	mu       sync.Mutex
	deps     *api.NotesDeps
	depsErr  error
	failedAt time.Time
	notesFn  = api.Notes(get)
)

// retryBuild is how long a failed build is answered from memory before trying
// again: a database that was briefly unreachable at cold start mustn't keep
// notes off for the instance's whole life.
const retryBuild = 30 * time.Second

// get builds the dependencies from env on first use and keeps them; missing env
// or a failed connect means notes are off (503) until a retry succeeds.
func get() (*api.NotesDeps, error) {
	mu.Lock()
	defer mu.Unlock()
	if deps != nil || (depsErr != nil && time.Since(failedAt) < retryBuild) {
		return deps, depsErr
	}
	deps, depsErr = build()
	if depsErr != nil {
		failedAt = time.Now()
	}
	return deps, depsErr
}

func build() (*api.NotesDeps, error) {
	drive := notes.DriveFromEnv() // GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REFRESH_TOKEN
	if drive == nil {
		return nil, errors.New("GOOGLE_* env not set")
	}
	signer, err := notes.NewSigner(os.Getenv("NOTES_TOKEN_SECRET"))
	if err != nil {
		return nil, err
	}
	url := os.Getenv("DATABASE_URL") // Neon's pooled URL
	if url == "" {
		return nil, errors.New("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pg, err := notes.OpenPG(ctx, url)
	if err != nil {
		return nil, err
	}
	origins := []string{"https://meowance.vercel.app"}
	for _, o := range strings.Split(os.Getenv("NOTES_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	return &api.NotesDeps{
		Svc:     &notes.Service{Repo: pg, Store: drive, Notify: api.ReportNotifier(api.ResendFromEnv(), origins[0])},
		Signer:  signer,
		Portal:  portal.NewClient(),
		Origins: origins,
	}, nil
}

// Handler is the entry point Vercel calls.
func Handler(w http.ResponseWriter, r *http.Request) { notesFn(w, r) }
