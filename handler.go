package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	maxContentBytes = 1 << 20
	maxAuthClients  = 1024
	maxAuthFailures = 5
	authLockout     = time.Minute
)

type authAttempt struct {
	failures int
	resetAt  time.Time
}

type Pool struct {
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updated_at"`
}

type saveRequest struct {
	Content string `json:"content"`
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	var mu sync.Mutex
	attempts := make(map[string]authAttempt)
	expected := sha256.Sum256([]byte("Bearer " + os.Getenv("APP_PASSWORD")))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		auth := r.Header.Get("Authorization")
		actual := sha256.Sum256([]byte(auth))
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}

		now := time.Now()
		mu.Lock()
		if _, tracked := attempts[host]; !tracked && len(attempts) >= maxAuthClients {
			for client, candidate := range attempts {
				if now.After(candidate.resetAt) {
					delete(attempts, client)
				}
			}
			if len(attempts) >= maxAuthClients {
				mu.Unlock()
				w.Header().Set("Retry-After", strconv.Itoa(int(authLockout.Seconds())))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
		}
		attempt := attempts[host]
		if !attempt.resetAt.IsZero() && now.After(attempt.resetAt) {
			delete(attempts, host)
			attempt = authAttempt{}
		}
		if attempt.failures >= maxAuthFailures {
			retryAfter := max(1, int(time.Until(attempt.resetAt).Seconds()))
			mu.Unlock()
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		mu.Unlock()

		if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			mu.Lock()
			attempt = attempts[host]
			if attempt.resetAt.IsZero() || now.After(attempt.resetAt) {
				attempt = authAttempt{resetAt: now.Add(authLockout)}
			}
			attempt.failures++
			attempts[host] = attempt
			mu.Unlock()
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		mu.Lock()
		delete(attempts, host)
		mu.Unlock()
		next(w, r)
	}
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
