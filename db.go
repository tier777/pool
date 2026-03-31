package main

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func InitDB() *sql.DB {
	db, err := sql.Open("sqlite", "./data.db")
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
