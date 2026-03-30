package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB() *sql.DB {
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
	return db
}
