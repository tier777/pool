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

	http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/api/pool", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			GetPoolHandler(db)(w, r)
			return
		}
		if r.Method == "POST" {
			SavePoolHandler(db)(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	fileServer := http.FileServer(http.Dir("./web"))
	http.Handle("/", fileServer)

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
