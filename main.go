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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.ListenAndServe(":"+port, nil)
}
