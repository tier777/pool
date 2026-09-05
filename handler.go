package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	// PIN is retained only for this expiring in-memory session, never in SQLite.
	pin       string
	csrfToken string
	expiresAt time.Time
}

type authManager struct {
	mu              sync.Mutex
	passwordHash    [sha256.Size]byte
	passwordSalt    string
	passwordEnabled bool
	db              *sql.DB
	attempts        map[string]authAttempt
	sessions        map[[sha256.Size]byte]session
	now             func() time.Time
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

type storedFile struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"created_at"`
}

func newAuthManager(password string) *authManager {
	salt, err := randomToken()
	if err != nil {
		panic(err)
	}
	return &authManager{
		passwordEnabled: true,
		passwordSalt:    salt,
		passwordHash:    passwordDigest(password, salt),
		attempts:        make(map[string]authAttempt),
		sessions:        make(map[[sha256.Size]byte]session),
		now:             time.Now,
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

	a.mu.Lock()
	actual := passwordDigest(req.Password, a.passwordSalt)
	if !a.passwordEnabled || subtle.ConstantTimeCompare(actual[:], a.passwordHash[:]) != 1 {
		a.mu.Unlock()
		a.recordPasswordFailure(host)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	defer a.mu.Unlock()
	delete(a.attempts, host)
	a.createSession(w, r, req.Password)
}

// createSession is called with mu held so a password change cannot race login.
func (a *authManager) createSession(w http.ResponseWriter, r *http.Request, pin string) {
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

	for candidate, value := range a.sessions {
		if now.After(value.expiresAt) {
			delete(a.sessions, candidate)
		}
	}
	if len(a.sessions) >= maxSessions {
		http.Error(w, "too many active sessions", http.StatusServiceUnavailable)
		return
	}
	a.sessions[key] = session{csrfToken: csrfToken, expiresAt: expires, pin: pin}

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
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, current, ok := a.currentSession(r)
	if !ok {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.passwordEnabled {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		a.createSession(w, r, "")
		return
	}
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

func FilesHandler(db *sql.DB, filesDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rows, err := db.Query(`SELECT id, name, size, created_at FROM files ORDER BY created_at DESC, id DESC`)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			defer rows.Close()
			files := []storedFile{}
			for rows.Next() {
				var file storedFile
				if err := rows.Scan(&file.ID, &file.Name, &file.Size, &file.CreatedAt); err != nil {
					http.Error(w, "db error", http.StatusInternalServerError)
					return
				}
				files = append(files, file)
			}
			if err := rows.Err(); err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(files)
		case http.MethodPost:
			uploadFile(w, r, db, filesDir)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func uploadFile(w http.ResponseWriter, r *http.Request, db *sql.DB, filesDir string) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		http.Error(w, "content type must be multipart/form-data", http.StatusUnsupportedMediaType)
		return
	}
	reader := multipart.NewReader(r.Body, params["boundary"])
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer part.Close()
	name := filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
	if name == "." || len(name) > 255 {
		http.Error(w, "invalid file name", http.StatusBadRequest)
		return
	}
	temp, err := os.CreateTemp(filesDir, ".upload-*")
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	size, copyErr := io.Copy(temp, part)
	closeErr := temp.Close()
	if copyErr != nil || closeErr != nil {
		http.Error(w, "upload failed", http.StatusBadRequest)
		return
	}
	contentType := part.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	now := time.Now().UnixMilli()
	result, err := db.Exec(`INSERT INTO files (name, content_type, size, created_at) VALUES (?, ?, ?, ?)`, name, contentType, size, now)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	id, _ := result.LastInsertId()
	if err := os.Rename(tempName, filepath.Join(filesDir, strconv.FormatInt(id, 10))); err != nil {
		db.Exec(`DELETE FROM files WHERE id = ?`, id)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(storedFile{ID: id, Name: name, Size: size, CreatedAt: now})
}

func FileHandler(db *sql.DB, filesDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/files/"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			var name, contentType string
			var size int64
			err := db.QueryRow(`SELECT name, content_type, size FROM files WHERE id = ?`, id).Scan(&name, &contentType, &size)
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			file, err := os.Open(filepath.Join(filesDir, strconv.FormatInt(id, 10)))
			if err != nil {
				http.Error(w, "storage error", http.StatusInternalServerError)
				return
			}
			defer file.Close()
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			if _, err := io.Copy(w, file); err != nil {
				return
			}
		case http.MethodDelete:
			if err := os.Remove(filepath.Join(filesDir, strconv.FormatInt(id, 10))); err != nil && !os.IsNotExist(err) {
				http.Error(w, "storage error", http.StatusInternalServerError)
				return
			}
			result, err := db.Exec(`DELETE FROM files WHERE id = ?`, id)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			if count, _ := result.RowsAffected(); count == 0 {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, DELETE")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func passwordDigest(password, salt string) [sha256.Size]byte {
	key, err := pbkdf2.Key(sha256.New, password, []byte(salt), 600000, sha256.Size)
	if err != nil {
		panic(err)
	}
	return [sha256.Size]byte(key)
}

func (a *authManager) loadSettings(db *sql.DB) error {
	a.db = db
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK (id = 1), password_enabled INTEGER NOT NULL, password_salt TEXT NOT NULL, password_hash BLOB NOT NULL)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO settings VALUES (1, ?, ?, ?)`, a.passwordEnabled, a.passwordSalt, a.passwordHash[:])
	if err != nil {
		return err
	}
	var hash []byte
	if err := db.QueryRow(`SELECT password_enabled, password_salt, password_hash FROM settings WHERE id = 1`).Scan(&a.passwordEnabled, &a.passwordSalt, &hash); err != nil {
		return err
	}
	if len(hash) != sha256.Size {
		return errors.New("invalid stored password hash")
	}
	copy(a.passwordHash[:], hash)
	return nil
}

func (a *authManager) settings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		key, _, ok := a.currentSession(r)
		a.mu.Lock()
		defer a.mu.Unlock()
		current, exists := a.sessions[key]
		if !ok || !exists || !a.now().Before(current.expiresAt) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Enabled bool   `json:"password_enabled"`
			PIN     string `json:"pin"`
		}{a.passwordEnabled, current.pin})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	host := clientHost(r)
	if allowed, retry := a.allowPasswordAttempt(host); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}
	var req struct {
		Enabled  *bool  `json:"password_enabled"`
		Current  string `json:"current_password"`
		Password string `json:"new_password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || req.Enabled == nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if *req.Enabled && (len(req.Password) < 16 || len(req.Password) > 1024) {
		http.Error(w, "password must be 16–1024 bytes", http.StatusBadRequest)
		return
	}
	key, _, ok := a.currentSession(r)
	a.mu.Lock()
	// Recheck under the mutation lock after any concurrent settings change.
	if _, exists := a.sessions[key]; !ok || !exists {
		a.mu.Unlock()
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if a.passwordEnabled {
		actual := passwordDigest(req.Current, a.passwordSalt)
		if subtle.ConstantTimeCompare(actual[:], a.passwordHash[:]) != 1 {
			a.mu.Unlock()
			a.recordPasswordFailure(host)
			http.Error(w, "wrong current password", http.StatusForbidden)
			return
		}
	}
	defer a.mu.Unlock()
	salt, err := randomToken()
	if err != nil {
		http.Error(w, "settings error", 500)
		return
	}
	hash := passwordDigest(req.Password, salt)
	if _, err := a.db.Exec(`UPDATE settings SET password_enabled = ?, password_salt = ?, password_hash = ? WHERE id = 1`, *req.Enabled, salt, hash[:]); err != nil {
		http.Error(w, "settings could not be saved", 500)
		return
	}
	a.passwordEnabled, a.passwordSalt, a.passwordHash = *req.Enabled, salt, hash
	current := a.sessions[key]
	current.pin = ""
	if *req.Enabled {
		current.pin = req.Password
	}
	a.sessions[key] = current
	for candidate := range a.sessions {
		if candidate != key {
			delete(a.sessions, candidate)
		}
	}
	clear(a.attempts)
	w.WriteHeader(http.StatusNoContent)
}
