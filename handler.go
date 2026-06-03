package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

type Pool struct {
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updated_at"`
}

type saveRequest struct {
	Content string `json:"content"`
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		auth := r.Header.Get("Authorization")
		expected := "Bearer " + os.Getenv("APP_PASSWORD")

		if len(auth) != len(expected) || subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
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
		var req saveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		now := time.Now().UnixMilli()
		_, err := db.Exec(`
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
