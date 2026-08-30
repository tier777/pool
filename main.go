package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if os.Getenv("APP_PASSWORD") == "" {
		log.Fatal("Environment variable APP_PASSWORD is not set")
	}

	db := InitDB()

	http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/api/pool", withAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			GetPoolHandler(db)(w, r)
		case http.MethodPost:
			SavePoolHandler(db)(w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	fileServer := http.FileServer(http.Dir("./web"))
	http.Handle("/", fileServer)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server started and listening on port %s\n", port)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           securityHeaders(http.DefaultServeMux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	err := server.ListenAndServe()

	if err != nil {
		log.Fatal("Server panic: ", err)
	}
}
