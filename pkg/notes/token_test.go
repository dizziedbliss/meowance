package notes

import (
	"strings"
	"testing"
	"time"
)

const secret = "0123456789abcdef0123456789abcdef" // 32 chars: long enough

func signerAt(t *testing.T, at time.Time) *Signer {
	t.Helper()
	s, err := NewSigner(secret)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return at }
	return s
}

func TestTokenRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	s := signerAt(t, now)
	tok, err := s.Issue("4MC23AI001")
	if err != nil {
		t.Fatal(err)
	}
	usn, ok := s.Verify(tok)
	if !ok || usn != "4MC23AI001" {
		t.Fatalf("Verify = %q, %v", usn, ok)
	}
}

func TestTokenLastsSevenDays(t *testing.T) {
	start := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	tok, _ := signerAt(t, start).Issue("4MC23AI001")

	if _, ok := signerAt(t, start.Add(7*24*time.Hour-time.Second)).Verify(tok); !ok {
		t.Error("a token should still work just before 7 days")
	}
	if _, ok := signerAt(t, start.Add(7*24*time.Hour)).Verify(tok); ok {
		t.Error("a token must be dead at 7 days")
	}
}

func TestTokenRejectsTampering(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	s := signerAt(t, now)
	tok, _ := s.Issue("4MC23AI001")
	other, _ := s.Issue("4MC23CS999")
	parts := strings.Split(tok, ".")
	otherParts := strings.Split(other, ".")

	bad := map[string]string{
		"someone else's payload, our signature": otherParts[0] + "." + parts[1],
		"our payload, someone else's signature": parts[0] + "." + otherParts[1],
		"signature flipped":                     parts[0] + "." + flip(parts[1]),
		"no dot":                                parts[0],
		"empty":                                 "",
		"garbage":                               "not.a.token",
		"three parts":                           tok + ".x",
	}
	for name, tk := range bad {
		if usn, ok := s.Verify(tk); ok {
			t.Errorf("%s: accepted as %q", name, usn)
		}
	}
}

func TestTokenFromAnotherSecretIsRejected(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	tok, _ := signerAt(t, now).Issue("4MC23AI001")
	other, _ := NewSigner("ffffffffffffffffffffffffffffffff")
	other.now = func() time.Time { return now }
	if _, ok := other.Verify(tok); ok {
		t.Error("a token signed with a different secret must not verify")
	}
}

func TestSignerNeedsARealSecret(t *testing.T) {
	for _, bad := range []string{"", "short", strings.Repeat("a", 31)} {
		if _, err := NewSigner(bad); err == nil {
			t.Errorf("NewSigner(%q) should fail", bad)
		}
	}
}

func TestIssueRefusesAUSNThatCouldForgeTheFormat(t *testing.T) {
	s := signerAt(t, time.Now())
	for _, usn := range []string{"", "4MC23AI001|9999999999", "4MC23 AI001", "4MC23AI001.x"} {
		if _, err := s.Issue(usn); err == nil {
			t.Errorf("Issue(%q) should fail", usn)
		}
	}
}

// flip changes one character so the signature is wrong but still valid base64.
func flip(sig string) string {
	b := []byte(sig)
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	return string(b)
}
