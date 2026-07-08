// Package session provides a stateless, HMAC-signed cookie session: bind
// an arbitrary subject string (a project ID, an admin identity, whatever)
// to an expiry, with no server-side store — nothing to clean up, nothing
// lost on restart except active logins. Shared by the dashboard (subject
// = project ID) and the admin panel (subject = a fixed admin identity),
// each with their own cookie name/path so the two never collide.
package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultTTL = 7 * 24 * time.Hour

var ErrInvalid = errors.New("session: invalid or expired session")

type Manager struct {
	secret     []byte
	cookieName string
	path       string
	ttl        time.Duration
}

// New creates a Manager whose cookies are scoped to path (e.g. "/dashboard"
// or "/admin") under cookieName — two Managers with different names/paths
// never interfere with each other even if they share the same secret.
func New(secret, cookieName, path string) *Manager {
	return &Manager{secret: []byte(secret), cookieName: cookieName, path: path, ttl: defaultTTL}
}

func (m *Manager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (m *Manager) issue(subject string) string {
	expiry := time.Now().Add(m.ttl).Unix()
	payload := subject + "|" + strconv.FormatInt(expiry, 10)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return encodedPayload + "." + m.sign(payload)
}

func (m *Manager) verify(token string) (subject string, err error) {
	dot := strings.LastIndex(token, ".")
	if dot < 0 {
		return "", ErrInvalid
	}
	encodedPayload, signature := token[:dot], token[dot+1:]

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", ErrInvalid
	}
	payload := string(payloadBytes)

	if !hmac.Equal([]byte(m.sign(payload)), []byte(signature)) {
		return "", ErrInvalid
	}

	fields := strings.SplitN(payload, "|", 2)
	if len(fields) != 2 {
		return "", ErrInvalid
	}
	expiry, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return "", ErrInvalid
	}
	return fields[0], nil
}

func (m *Manager) SetCookie(w http.ResponseWriter, subject string) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    m.issue(subject),
		Path:     m.path,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(m.ttl.Seconds()),
	})
}

func (m *Manager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     m.path,
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// SubjectFromRequest returns the authenticated subject, or ErrInvalid if
// there's no valid session cookie.
func (m *Manager) SubjectFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return "", ErrInvalid
	}
	return m.verify(cookie.Value)
}
