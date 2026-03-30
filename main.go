package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var pools = make(map[string]string)

func main() {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		panic(err)
	}
	query := `
CREATE TABLE IF NOT EXISTS pools (
		code TEXT PRIMARY KEY,
		content TEXT
);
`
	_, err = db.Exec(query)
	if err != nil {
		panic(err)
	}
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/pool/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		code := parts[2]
		if code == "" {
			http.Error(w, "code is required", http.StatusBadRequest)
			return
		}
		if r.Method == "POST" {
			var req PoolRequest
			error := json.NewDecoder(r.Body).Decode(&req)
			if error != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			_, err := db.Exec(
				"INSERT OR REPLACE INTO pools (code, content) VALUES (?, ?)",
				code,
				req.Content,
			)
			if err != nil {
				http.Error(w, "failed to save", http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, "saved")
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
			resp := PoolRequest{Content: content}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		fmt.Fprint(w, "unsupported method")
	})
	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}

type PoolRequest struct {
	Content string `json:"content"`
}
