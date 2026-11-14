// Command notesadmin is how Max takes a reported note down: it deletes the note's file from
// the store (Google Drive) and its row from the database.
//
//	go run ./cmd/notesadmin remove <note id>
//
// It shows the note first and makes you type its id back before it deletes anything.
//
// The note id is in the report email. It needs the same env as the live function:
// DATABASE_URL, and GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and GOOGLE_REFRESH_TOKEN
// (the Google ones are Production-only secrets, so put them in your shell first).
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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

// confirm asks for the note id to be typed back; anything else cancels.
func confirm(in io.Reader, out io.Writer, id string) bool {
	fmt.Fprintf(out, "type the note id (%s) again to delete it: ", id)
	line, _ := bufio.NewReader(in).ReadString('\n')
	return id != "" && strings.TrimSpace(line) == id
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
	n, err := pg.Get(ctx, id)
	if errors.Is(err, notes.ErrNotFound) {
		return fmt.Errorf("no note with id %q (already removed?)", id)
	}
	if err != nil {
		return err
	}
	// show what is about to go, so a mangled or look-alike id can't delete the wrong note unnoticed
	fmt.Printf("about to delete:\n  id:     %s\n  course: %s\n  title:  %q\n  status: %s, %d votes\n", n.ID, n.Code, n.Title, n.Status, n.Votes)
	if !confirm(os.Stdin, os.Stdout, id) {
		return errors.New("not confirmed, nothing deleted")
	}
	svc := &notes.Service{Repo: pg, Store: drive}
	if err := svc.Remove(ctx, id); err != nil {
		return err
	}
	fmt.Println("removed", id, "(file and row)")
	return nil
}
