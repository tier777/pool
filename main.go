package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	if os.Getenv("APP_PASSWORD") == "" {
		log.Fatal("Environment variable APP_PASSWORD is not set")
	}

	db := InitDB()

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/pool/", withAuth(PoolHandler(db)))
	http.HandleFunc("/pools", withAuth(ClearHandler(db)))

	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
