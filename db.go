package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

func InitDB(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)

	query := `
		CREATE TABLE IF NOT EXISTS pool (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			content TEXT,
			updated_at INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			content_type TEXT NOT NULL,
			size INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		);
		`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal("failed to create table:", err)
	}

	migrateUpdatedAt(db)
	if err := os.Chmod(path, 0o600); err != nil {
		log.Fatal("failed to secure database permissions:", err)
	}

	return db
}

func migrateUpdatedAt(db *sql.DB) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('pool') WHERE name = 'updated_at'`,
	).Scan(&count)
	if err != nil {
		log.Fatal("failed to inspect pool table:", err)
	}
	if count > 0 {
		return
	}

	_, err = db.Exec(`ALTER TABLE pool ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0`)
	if err != nil {
		log.Fatal("failed to add updated_at column:", err)
	}

	now := time.Now().UnixMilli()
	_, err = db.Exec(`UPDATE pool SET updated_at = ? WHERE updated_at = 0`, now)
	if err != nil {
		log.Fatal("failed to backfill updated_at:", err)
	}
}
