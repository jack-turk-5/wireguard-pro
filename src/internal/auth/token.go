// Package auth issues and verifies HMAC-signed, timestamped bearer tokens,
// replacing the original app's itsdangerous URLSafeTimedSerializer usage.
// The wire format is new, not itsdangerous-compatible -- fine since this is
// an atomic redeploy, not a rolling upgrade: no token issued by the old
// backend needs to still validate against the new one.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrExpired is returned by Verify for a well-formed, correctly-signed
	// token whose max age has elapsed.
	ErrExpired = errors.New("auth: token has expired")
	// ErrInvalid is returned by Verify for a malformed token or one whose
	// signature doesn't match.
	ErrInvalid = errors.New("auth: invalid token")
)

// Tokens issues and verifies bearer tokens tied to a single secret key and
// max age (the original app used 1800s / 30 minutes).
type Tokens struct {
	secret []byte
	maxAge time.Duration
}

// New returns a Tokens issuer/verifier for the given secret key and max age.
func New(secretKey string, maxAge time.Duration) *Tokens {
	return &Tokens{secret: []byte(secretKey), maxAge: maxAge}
}

// Generate issues a token for username, valid from now for t's maxAge.
func (t *Tokens) Generate(username string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(username))
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	return payload + "." + ts + "." + t.sign(payload, ts)
}

// Verify checks a token's signature and expiry, returning the username it
// was issued for.
func (t *Tokens) Verify(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", ErrInvalid
	}
	payload, ts, sig := parts[0], parts[1], parts[2]

	expected := t.sign(payload, ts)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", ErrInvalid
	}

	issued, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return "", ErrInvalid
	}
	if time.Since(time.Unix(issued, 0)) > t.maxAge {
		return "", ErrExpired
	}

	usernameBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", ErrInvalid
	}
	return string(usernameBytes), nil
}

func (t *Tokens) sign(payload, ts string) string {
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(payload + "." + ts))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
