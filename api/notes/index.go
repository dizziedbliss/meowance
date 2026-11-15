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
	once    sync.Once
	deps    *api.NotesDeps
	depsErr error
	notesFn = api.Notes(get)
)

// get builds the dependencies on the first call from env; missing env means notes stay off (503).
func get() (*api.NotesDeps, error) {
	once.Do(func() { deps, depsErr = build() })
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
