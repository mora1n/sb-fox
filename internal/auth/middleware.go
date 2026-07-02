package auth

import (
	"context"
	"net/http"
	"time"
)

type ctxKey string

const subjectKey ctxKey = "auth.subject"

// SetSessionCookie writes the session cookie for subject on the response.
func (a *Authenticator) SetSessionCookie(w http.ResponseWriter, subject string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    a.Issue(subject, time.Now()),
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(SessionTTL),
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// Subject returns the authenticated subject stored in the request context.
func Subject(r *http.Request) (string, bool) {
	sub, ok := r.Context().Value(subjectKey).(string)
	return sub, ok
}

// RequireAuth is middleware that rejects requests without a valid session.
func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(CookieName)
		if err != nil {
			unauthorized(w)
			return
		}
		subject, err := a.Verify(cookie.Value, time.Now())
		if err != nil {
			unauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), subjectKey, subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"data":null,"error":{"code":"unauthorized","message":"authentication required"}}`))
}
