package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
)

type Pool struct {
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

func GetPoolHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var content string

		err := db.QueryRow("SELECT content FROM pool WHERE id = 1").Scan(&content)
		if err == sql.ErrNoRows {
			content = ""
		} else if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Pool{Content: content})
	}
}

func SavePoolHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var p Pool
		json.NewDecoder(r.Body).Decode(&p)

		_, err := db.Exec(`
			INSERT INTO pool (id, content)
			VALUES (1, ?)
			ON CONFLICT(id) DO UPDATE SET content = excluded.content
		`, p.Content)

		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		w.Write([]byte("ok"))
	}
}
