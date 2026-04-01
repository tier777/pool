package main

import (
	"database/sql"

	"log"

	_ "modernc.org/sqlite"
)

func InitDB() *sql.DB {
	db, err := sql.Open("sqlite", "./data.db")
	if err != nil {
		panic(err)
	}
	query := `
		CREATE TABLE IF NOT EXISTS pool (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			content TEXT
		);
		`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal("failed to create table:", err)
	}
	return db
}
