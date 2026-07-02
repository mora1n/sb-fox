// Package auth provides single-admin authentication: bcrypt password
// verification plus stateless HMAC-signed session cookies. No external JWT
// dependency — sessions are signed with crypto/hmac over the standard library.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SessionTTL is how long an issued session remains valid.
const SessionTTL = 7 * 24 * time.Hour

// CookieName is the session cookie name.
const CookieName = "sb-fox_session"

// Authenticator issues and verifies session tokens using a server secret.
type Authenticator struct {
	secret []byte
}

// New returns an Authenticator keyed by secret (must be non-empty).
func New(secret []byte) *Authenticator {
	return &Authenticator{secret: secret}
}

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// VerifyPassword reports whether password matches the bcrypt hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Issue creates a signed session token for subject valid until now+SessionTTL.
// Token format: base64(subject):expiryUnix:hexHMAC.
func (a *Authenticator) Issue(subject string, now time.Time) string {
	exp := now.Add(SessionTTL).Unix()
	payload := encodePayload(subject, exp)
	return payload + ":" + a.sign(payload)
}

// Verify validates a session token and returns the subject if valid.
func (a *Authenticator) Verify(token string, now time.Time) (string, error) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return "", errors.New("auth: malformed token")
	}
	payload := parts[0] + ":" + parts[1]
	if !hmac.Equal([]byte(a.sign(payload)), []byte(parts[2])) {
		return "", errors.New("auth: bad signature")
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", errors.New("auth: bad expiry")
	}
	if now.Unix() > exp {
		return "", errors.New("auth: expired")
	}
	sub, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("auth: bad subject")
	}
	return string(sub), nil
}

func encodePayload(subject string, exp int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(subject)) + ":" + strconv.FormatInt(exp, 10)
}

func (a *Authenticator) sign(payload string) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
