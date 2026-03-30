package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type PoolRequest struct {
	Content string `json:"content"`
}

type PoolResponse struct {
	Content string `json:"content"`
}

func PoolHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		auth := r.Header.Get("Authorization")
		expected := "Bearer " + os.Getenv("APP_PASSWORD")

		if auth != expected {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(r.URL.Path, "/")

		if len(parts) < 3 {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		code := parts[2]

		if r.Method == "POST" {
			var req PoolRequest

			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}

			_, err = db.Exec(
				"INSERT OR REPLACE INTO pools (code, content) VALUES (?, ?)",
				code,
				req.Content,
			)

			if err != nil {
				http.Error(w, "failed to save", http.StatusInternalServerError)
				return
			}

			w.Write([]byte("saved"))
			return
		}

		if r.Method == "GET" {
			var content string

			err := db.QueryRow(
				"SELECT content FROM pools WHERE code = ?",
				code,
			).Scan(&content)

			if err == sql.ErrNoRows {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}

			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			resp := PoolResponse{Content: content}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func ClearHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != "DELETE" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// простое подтверждение через query
		confirm := r.URL.Query().Get("confirm")

		if confirm != "yes" {
			http.Error(w, "confirmation required (?confirm=yes)", http.StatusBadRequest)
			return
		}

		_, err := db.Exec("DELETE FROM pools")
		if err != nil {
			http.Error(w, "failed to clear", http.StatusInternalServerError)
			return
		}

		w.Write([]byte("cleared"))
	}
}
