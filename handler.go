package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	maxContentBytes = 1 << 20
	maxLoginBytes   = 4 << 10
	maxAuthClients  = 1024
	maxAuthFailures = 5
	maxSessions     = 1024
	authLockout     = time.Minute
	sessionDuration = 8 * time.Hour
	sessionCookie   = "pool_session"
)

type authAttempt struct {
	failures int
	resetAt  time.Time
}

type session struct {
	csrfToken string
	expiresAt time.Time
}

type authManager struct {
	mu           sync.Mutex
	passwordHash [sha256.Size]byte
	attempts     map[string]authAttempt
	sessions     map[[sha256.Size]byte]session
	now          func() time.Time
}

type Pool struct {
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updated_at"`
}

type saveRequest struct {
	Content string `json:"content"`
}

type loginRequest struct {
	Password string `json:"password"`
}

type sessionResponse struct {
	CSRFToken string `json:"csrf_token"`
}

func newAuthManager(password string) *authManager {
	return &authManager{
		passwordHash: sha256.Sum256([]byte(password)),
		attempts:     make(map[string]authAttempt),
		sessions:     make(map[[sha256.Size]byte]session),
		now:          time.Now,
	}
}

func clientHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *authManager) allowPasswordAttempt(host string) (bool, int) {
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, tracked := a.attempts[host]; !tracked && len(a.attempts) >= maxAuthClients {
		for client, candidate := range a.attempts {
			if now.After(candidate.resetAt) {
				delete(a.attempts, client)
			}
		}
		if len(a.attempts) >= maxAuthClients {
			return false, int(authLockout.Seconds())
		}
	}

	attempt := a.attempts[host]
	if !attempt.resetAt.IsZero() && now.After(attempt.resetAt) {
		delete(a.attempts, host)
		return true, 0
	}
	if attempt.failures >= maxAuthFailures {
		return false, max(1, int(attempt.resetAt.Sub(now).Seconds()))
	}
	return true, 0
}

func (a *authManager) recordPasswordFailure(host string) {
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()
	attempt := a.attempts[host]
	if attempt.resetAt.IsZero() || now.After(attempt.resetAt) {
		attempt = authAttempt{resetAt: now.Add(authLockout)}
	}
	attempt.failures++
	a.attempts[host] = attempt
}

func randomToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

func secureCookie(r *http.Request) bool {
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	host := r.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return host != "localhost" && !net.ParseIP(host).IsLoopback()
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *authManager) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	host := clientHost(r)
	if allowed, retryAfter := a.allowPasswordAttempt(host); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBytes)
	var req loginRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	actual := sha256.Sum256([]byte(req.Password))
	if subtle.ConstantTimeCompare(actual[:], a.passwordHash[:]) != 1 {
		a.recordPasswordFailure(host)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionToken, err := randomToken()
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	csrfToken, err := randomToken()
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	now := a.now()
	expires := now.Add(sessionDuration)
	key := sha256.Sum256([]byte(sessionToken))

	a.mu.Lock()
	delete(a.attempts, host)
	for candidate, value := range a.sessions {
		if now.After(value.expiresAt) {
			delete(a.sessions, candidate)
		}
	}
	if len(a.sessions) >= maxSessions {
		a.mu.Unlock()
		http.Error(w, "too many active sessions", http.StatusServiceUnavailable)
		return
	}
	a.sessions[key] = session{csrfToken: csrfToken, expiresAt: expires}
	a.mu.Unlock()

	setSessionCookie(w, r, sessionToken, expires)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionResponse{CSRFToken: csrfToken})
}

func (a *authManager) currentSession(r *http.Request) ([sha256.Size]byte, session, bool) {
	var emptyKey [sha256.Size]byte
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return emptyKey, session{}, false
	}
	key := sha256.Sum256([]byte(cookie.Value))
	a.mu.Lock()
	defer a.mu.Unlock()
	current, ok := a.sessions[key]
	if !ok || !a.now().Before(current.expiresAt) {
		delete(a.sessions, key)
		return emptyKey, session{}, false
	}
	return key, current, true
}

func (a *authManager) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, current, ok := a.currentSession(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			actual := sha256.Sum256([]byte(r.Header.Get("X-CSRF-Token")))
			expected := sha256.Sum256([]byte(current.csrfToken))
			if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}

func (a *authManager) sessionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, current, _ := a.currentSession(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionResponse{CSRFToken: current.csrfToken})
}

func (a *authManager) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key, _, _ := a.currentSession(r)
	a.mu.Lock()
	delete(a.sessions, key)
	a.mu.Unlock()
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; connect-src 'self'; form-action 'none'; frame-ancestors 'none'; img-src 'self'; object-src 'none'; script-src 'self'; style-src 'self'")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		next.ServeHTTP(w, r)
	})
}

func writePool(w http.ResponseWriter, p Pool) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func readPoolFromDB(db *sql.DB) (Pool, error) {
	var p Pool
	err := db.QueryRow("SELECT content, updated_at FROM pool WHERE id = 1").Scan(&p.Content, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return Pool{}, nil
	}
	return p, err
}

func GetPoolHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := readPoolFromDB(db)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		writePool(w, p)
	}
}

func SavePoolHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxContentBytes)
		var req saveRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		now := time.Now().UnixMilli()
		_, err = db.Exec(`
			INSERT INTO pool (id, content, updated_at)
			VALUES (1, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				content = excluded.content,
				updated_at = excluded.updated_at
		`, req.Content, now)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		writePool(w, Pool{Content: req.Content, UpdatedAt: now})
	}
}
