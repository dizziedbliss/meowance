package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Feedback is the "tell max" box: POST /api/feedback {usn, message} emails the
// message to Max with the student's USN. The USN is the one the logged-in
// browser sends; it isn't checked against the portal.

// Mailer sends one feedback email.
type Mailer interface {
	Send(ctx context.Context, usn, message string) error
}

const (
	maxFeedback       = 1000 // characters
	feedbackPerIP     = 5    // per hour, per IP
	feedbackAll       = 100  // per hour, per instance: the inbox's flood wall
	feedbackWindow    = time.Hour
	CodeFeedbackOff   = "feedback_off" // no mailer configured
	CodeFeedbackError = "feedback_failed"
)

type FeedbackRequest struct {
	USN     string `json:"usn"`
	Message string `json:"message"`
}

// Feedback returns the POST /api/feedback handler. A nil mailer answers 503.
func Feedback(m Mailer) http.HandlerFunc {
	return feedback(m, time.Now)
}

func feedback(m Mailer, now func() time.Time) http.HandlerFunc {
	perIP := newGuard(feedbackPerIP, feedbackWindow, now)
	all := newGuard(feedbackAll, feedbackWindow, now)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeErr(w, http.StatusMethodNotAllowed, CodeBadInput)
			return
		}
		if m == nil {
			writeErr(w, http.StatusServiceUnavailable, CodeFeedbackOff)
			return
		}
		var req FeedbackRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, CodeBadInput)
			return
		}
		usn := strings.ToUpper(strings.TrimSpace(req.USN))
		msg := strings.TrimSpace(req.Message)
		if !reUSN.MatchString(usn) || msg == "" || utf8.RuneCountInString(msg) > maxFeedback {
			writeErr(w, http.StatusBadRequest, CodeBadInput)
			return
		}

		ip := clientIP(r)
		t1, ok, wait := perIP.begin(ip)
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			writeErr(w, http.StatusTooManyRequests, CodeTooManyTries)
			return
		}
		t2, ok, wait := all.begin("all")
		if !ok {
			perIP.release(t1)
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			writeErr(w, http.StatusTooManyRequests, CodeTooManyTries)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := m.Send(ctx, usn, msg); err != nil {
			perIP.release(t1) // a failed send doesn't use up anyone's tries
			all.release(t2)
			log.Printf("feedback send failed: %v", err)
			writeErr(w, http.StatusBadGateway, CodeFeedbackError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}

// Resend sends feedback through Resend's email API (plain text only).
type Resend struct {
	APIKey   string
	From, To string // "meowance <feedback@…>", the inbox
	BaseURL  string // https://api.resend.com
	HTTP     *http.Client
}

// ResendFromEnv is the mailer for RESEND_API_KEY, FEEDBACK_FROM and
// FEEDBACK_TO, or nil when any is missing (the box then answers 503).
func ResendFromEnv() Mailer {
	key, from, to := os.Getenv("RESEND_API_KEY"), os.Getenv("FEEDBACK_FROM"), os.Getenv("FEEDBACK_TO")
	if key == "" || from == "" || to == "" {
		return nil
	}
	return &Resend{APIKey: key, From: from, To: to, BaseURL: "https://api.resend.com", HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (s *Resend) Send(ctx context.Context, usn, message string) error {
	body, _ := json.Marshal(map[string]any{
		"from":    s.From,
		"to":      []string{s.To},
		"subject": "meowance feedback from " + usn,
		"text":    fmt.Sprintf("from: %s (as sent by the browser, not verified)\n\n%s\n", usn, message),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("resend: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
