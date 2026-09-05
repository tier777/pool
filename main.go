package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	password, err := loadPassword()
	if err != nil {
		log.Fatal(err)
	}

	dataPath := os.Getenv("DATA_PATH")
	if dataPath == "" {
		dataPath = "./data.db"
	}
	db := InitDB(dataPath)
	defer db.Close()
	filesPath := dataPath + ".files"
	if err := os.MkdirAll(filesPath, 0o700); err != nil {
		log.Fatal("failed to create files directory: ", err)
	}
	auth := newAuthManager(password)
	if err := auth.loadSettings(db); err != nil {
		log.Fatal("failed to load settings: ", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		fmt.Fprint(w, "pong")
	})
	mux.HandleFunc("/api/login", auth.login)
	mux.HandleFunc("/api/session", auth.sessionInfo)
	mux.HandleFunc("/api/settings", auth.withSession(auth.settings))
	mux.HandleFunc("/api/logout", auth.withSession(auth.logout))
	mux.HandleFunc("/api/pool", auth.withSession(func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/api/files", auth.withSession(FilesHandler(db, filesPath)))
	mux.HandleFunc("/api/files/", auth.withSession(FileHandler(db, filesPath)))

	fileServer := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fileServer)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	bindAddress := os.Getenv("BIND_ADDRESS")
	if bindAddress == "" {
		bindAddress = "127.0.0.1"
	}
	address := net.JoinHostPort(bindAddress, port)

	server := &http.Server{
		Addr:              address,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	log.Printf("Server started and listening on %s", address)
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Server failed: ", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			server.Close()
		}
	}
}

func loadPassword() (string, error) {
	password := os.Getenv("APP_PASSWORD")
	if path := os.Getenv("APP_PASSWORD_FILE"); path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read APP_PASSWORD_FILE: %w", err)
		}
		password = strings.TrimSpace(string(contents))
	}
	if len(password) < 16 {
		return "", errors.New("APP_PASSWORD or APP_PASSWORD_FILE must contain at least 16 characters")
	}
	return password, nil
}
