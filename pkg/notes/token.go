// Package notes holds the pure pieces of notes sharing: the signed token a
// verified student carries, and the funny anonymous label a note shows.
package notes

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// TokenLife is how long a verified login keeps working for posting, voting and
// reporting, so a midnight portal outage only blocks people who haven't logged in this week.
const TokenLife = 7 * 24 * time.Hour

const (
	minSecret = 32 // bytes of secret: shorter ones are guessable
	version   = "v1"
)

var enc = base64.RawURLEncoding

// Signer issues and checks tokens. Nothing is stored: a token is its own proof,
// signed with a secret only the server has.
type Signer struct {
	secret []byte
	now    func() time.Time
}

// NewSigner needs a secret of at least 32 characters.
func NewSigner(secret string) (*Signer, error) {
	if len(secret) < minSecret {
		return nil, errors.New("notes: token secret must be at least 32 characters")
	}
	return &Signer{secret: []byte(secret), now: time.Now}, nil
}

// Issue makes a token for a USN that the portal has just verified.
func (s *Signer) Issue(usn string) (string, error) {
	if !plain(usn) {
		return "", errors.New("notes: not a USN")
	}
	payload := strings.Join([]string{version, usn, strconv.FormatInt(s.now().Add(TokenLife).Unix(), 10)}, "|")
	return enc.EncodeToString([]byte(payload)) + "." + enc.EncodeToString(s.mac(payload)), nil
}

// Verify returns the USN a token was issued for, if it is genuine and unexpired.
func (s *Signer) Verify(token string) (usn string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false
	}
	payload, err := enc.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	sig, err := enc.DecodeString(parts[1])
	if err != nil || !hmac.Equal(sig, s.mac(string(payload))) {
		return "", false
	}
	f := strings.Split(string(payload), "|")
	if len(f) != 3 || f[0] != version || !plain(f[1]) {
		return "", false
	}
	exp, err := strconv.ParseInt(f[2], 10, 64)
	if err != nil || !s.now().Before(time.Unix(exp, 0)) {
		return "", false
	}
	return f[1], true
}

func (s *Signer) mac(payload string) []byte {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// plain reports whether s is non-empty letters and digits only, so it can't
// smuggle the "|" or "." separators into a token.
func plain(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}
