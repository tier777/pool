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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "server is active")
	})
	http.HandleFunc("/pool/", withAuth(PoolHandler(db)))
	http.HandleFunc("/pools", withAuth(PoolsHandler(db)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server started and listening on port %s\n", port)

	err := http.ListenAndServe(":"+port, nil)

	if err != nil {
		log.Fatal("Server panic: ", err)
	}
}
