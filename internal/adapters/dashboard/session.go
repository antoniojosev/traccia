// Package dashboard is Traccia's embedded, server-rendered dashboard:
// HTMX + html/template, no frontend build step, so it stays inside the
// "one binary" story the tracking script already tells. A plugin (see the
// planned goja runtime) extends this by declaring a panel spec the server
// renders — it never ships its own frontend JS.
package dashboard

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

const (
	sessionCookieName = "traccia_session"
	sessionTTL        = 7 * 24 * time.Hour
)

var ErrInvalidSession = errors.New("dashboard: invalid or expired session")

// SessionManager issues and verifies HMAC-signed session cookies. A
// session is just a signed (project_id, expiry) pair — there's no
// server-side session store, so nothing to clean up and nothing to lose
// on restart except active logins.
type SessionManager struct {
	secret []byte
}

func NewSessionManager(secret string) *SessionManager {
	return &SessionManager{secret: []byte(secret)}
}

func (sm *SessionManager) sign(payload string) string {
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (sm *SessionManager) issue(projectID string) string {
	expiry := time.Now().Add(sessionTTL).Unix()
	payload := projectID + "|" + strconv.FormatInt(expiry, 10)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return encodedPayload + "." + sm.sign(payload)
}

func (sm *SessionManager) verify(token string) (projectID string, err error) {
	dot := strings.LastIndex(token, ".")
	if dot < 0 {
		return "", ErrInvalidSession
	}
	encodedPayload, signature := token[:dot], token[dot+1:]

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", ErrInvalidSession
	}
	payload := string(payloadBytes)

	if !hmac.Equal([]byte(sm.sign(payload)), []byte(signature)) {
		return "", ErrInvalidSession
	}

	fields := strings.SplitN(payload, "|", 2)
	if len(fields) != 2 {
		return "", ErrInvalidSession
	}
	expiry, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return "", ErrInvalidSession
	}
	return fields[0], nil
}

func (sm *SessionManager) SetCookie(w http.ResponseWriter, projectID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sm.issue(projectID),
		Path:     "/dashboard",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

func (sm *SessionManager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/dashboard",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// ProjectIDFromRequest returns the authenticated project ID, or
// ErrInvalidSession if there's no valid session cookie.
func (sm *SessionManager) ProjectIDFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", ErrInvalidSession
	}
	return sm.verify(cookie.Value)
}
