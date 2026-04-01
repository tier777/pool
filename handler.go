package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

		if auth != expected {
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

func MainHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>Pool</title>
</head>
<body>

<h2>Pool</h2>

<textarea id="text" style="width:100%;height:200px"></textarea>
<br>

<button onclick="save()">Save</button>
<button onclick="copyText()">Copy</button>

<script>

async function load() {
			const res = await fetch('/pool')
			const data = await res.json()
			document.getElementById('text').value = data.content
}

async function save() {
			const content = document.getElementById('text').value

			await fetch('/pool', {
				method: 'POST',
				headers: {'Content-Type': 'application/json'},
				body: JSON.stringify({content})
			})
}

function copyText() {
			const text = document.getElementById('text')
			text.select()
			document.execCommand('copy')
}

load()

</script>

</body>
</html>
`)
	}
}
