package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeMailer struct {
	sent []string
	err  error
}

func (m *fakeMailer) Send(_ context.Context, usn, message string) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, usn+": "+message)
	return nil
}

func postFeedback(h http.HandlerFunc, ip, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("X-Real-IP", ip)
	h(rec, req)
	return rec
}

func TestFeedbackSends(t *testing.T) {
	m := &fakeMailer{}
	rec := postFeedback(Feedback(m), "1.1.1.1", `{"usn":" 4mc00aa000 ","message":"  timetable's wrong for batch 2  "}`)
	if rec.Code != http.StatusOK || len(m.sent) != 1 || m.sent[0] != "4MC00AA000: timetable's wrong for batch 2" {
		t.Fatalf("status %d, sent %q", rec.Code, m.sent)
	}
}

func TestFeedbackRejects(t *testing.T) {
	for name, body := range map[string]string{
		"not mce":   `{"usn":"1RV22CS001","message":"hi"}`,
		"empty":     `{"usn":"4MC00AA000","message":"   "}`,
		"too long":  `{"usn":"4MC00AA000","message":"` + strings.Repeat("a", maxFeedback+1) + `"}`,
		"junk body": `nope`,
	} {
		m := &fakeMailer{}
		if rec := postFeedback(Feedback(m), "1.1.1.1", body); rec.Code != http.StatusBadRequest || len(m.sent) != 0 {
			t.Errorf("%s: status %d, sent %d", name, rec.Code, len(m.sent))
		}
	}
}

func TestFeedbackOffWithoutMailer(t *testing.T) {
	rec := postFeedback(Feedback(nil), "1.1.1.1", `{"usn":"4MC00AA000","message":"hi"}`)
	var got map[string]string
	json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusServiceUnavailable || got["error"] != CodeFeedbackOff {
		t.Fatalf("got %d %v", rec.Code, got)
	}
}

func TestFeedbackLimitsPerIPAndFailuresDontCount(t *testing.T) {
	m := &fakeMailer{err: errors.New("resend down")}
	clock := newClock()
	h := feedback(m, clock.now)
	for range feedbackPerIP + 2 {
		if rec := postFeedback(h, "1.1.1.1", `{"usn":"4MC00AA000","message":"hi"}`); rec.Code != http.StatusBadGateway {
			t.Fatalf("failed send: status %d, want 502 (and no try used)", rec.Code)
		}
	}
	m.err = nil
	for i := range feedbackPerIP {
		if rec := postFeedback(h, "1.1.1.1", `{"usn":"4MC00AA000","message":"hi"}`); rec.Code != http.StatusOK {
			t.Fatalf("send %d: status %d", i+1, rec.Code)
		}
	}
	if rec := postFeedback(h, "1.1.1.1", `{"usn":"4MC00AA000","message":"hi"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over the per-ip limit: status %d, want 429", rec.Code)
	}
	if rec := postFeedback(h, "2.2.2.2", `{"usn":"4MC00AA000","message":"hi"}`); rec.Code != http.StatusOK {
		t.Fatalf("another ip: status %d", rec.Code)
	}
	clock.t = clock.t.Add(feedbackWindow + time.Minute)
	if rec := postFeedback(h, "1.1.1.1", `{"usn":"4MC00AA000","message":"hi"}`); rec.Code != http.StatusOK {
		t.Fatalf("after an hour: status %d", rec.Code)
	}
}

func TestFeedbackCapsTheInbox(t *testing.T) {
	m := &fakeMailer{}
	h := feedback(m, newClock().now)
	for i := range feedbackAll {
		postFeedback(h, fmt.Sprintf("10.0.%d.%d", i/250, i%250), `{"usn":"4MC00AA000","message":"hi"}`)
	}
	sent := len(m.sent)
	if rec := postFeedback(h, "9.9.9.9", `{"usn":"4MC00AA000","message":"hi"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("after %d mails from many ips: status %d, want 429", sent, rec.Code)
	}
}

func TestResendRequest(t *testing.T) {
	var got struct {
		auth, ctype string
		body        map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.auth, got.ctype = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got.body)
		if r.URL.Path != "/emails" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		io.WriteString(w, `{"id":"x"}`)
	}))
	defer srv.Close()
	s := &Resend{APIKey: "re_test", From: "meowance <feedback@example.com>", To: "inbox@example.com", BaseURL: srv.URL, HTTP: srv.Client()}
	if err := s.Send(context.Background(), "4MC00AA000", "<b>hi</b>"); err != nil {
		t.Fatal(err)
	}
	if got.auth != "Bearer re_test" || got.ctype != "application/json" {
		t.Errorf("headers: %q %q", got.auth, got.ctype)
	}
	if got.body["subject"] != "meowance feedback from 4MC00AA000" || got.body["text"] != "from: 4MC00AA000\n\n<b>hi</b>\n" || got.body["html"] != nil {
		t.Errorf("body: %v (plain text only, never html)", got.body)
	}
	if to, _ := got.body["to"].([]any); len(to) != 1 || to[0] != "inbox@example.com" {
		t.Errorf("to: %v", got.body["to"])
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"message":"domain not verified"}`)
	}))
	defer bad.Close()
	s.BaseURL, s.HTTP = bad.URL, bad.Client()
	if err := s.Send(context.Background(), "4MC00AA000", "hi"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v, want the status in it", err)
	}
}
