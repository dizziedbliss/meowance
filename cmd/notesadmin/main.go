// Command notesadmin is how Max takes a reported note down: it deletes the note's file from
// the store (Google Drive) and its row from the database.
//
//	go run ./cmd/notesadmin remove <note id>
//
// The note id is in the report email. It needs the same env as the live function:
// DATABASE_URL, and GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and GOOGLE_REFRESH_TOKEN
// (the Google ones are Production-only secrets, so put them in your shell first).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dizziedbliss/meowance/pkg/notes"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "remove" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/notesadmin remove <note id>")
		os.Exit(2)
	}
	if err := remove(os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "failed:", err)
		os.Exit(1)
	}
}

func remove(id string) error {
	drive := notes.DriveFromEnv()
	if drive == nil {
		return errors.New("set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and GOOGLE_REFRESH_TOKEN first")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("set DATABASE_URL first (vercel env pull has it)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pg, err := notes.OpenPG(ctx, url)
	if err != nil {
		return err
	}
	defer pg.Close()
	svc := &notes.Service{Repo: pg, Store: drive}
	if err := svc.Remove(ctx, id); err != nil {
		if errors.Is(err, notes.ErrNotFound) {
			return fmt.Errorf("no note with id %q (already removed?)", id)
		}
		return err
	}
	fmt.Println("removed", id, "(file and row)")
	return nil
}
