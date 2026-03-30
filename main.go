package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

var pools = make(map[string]string)

func main() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/pool/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		code := parts[2]
		if code == "" {
			http.Error(w, "code is required", http.StatusBadRequest)
			return
		}
		if r.Method == "POST" {
			var req PoolRequest
			error := json.NewDecoder(r.Body).Decode(&req)
			if error != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			pools[code] = req.Content
			fmt.Fprint(w, "saved")
			return
		}
		if r.Method == "GET" {
			value, ok := pools[code]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			resp := PoolRequest{Content: value}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		fmt.Fprint(w, "unsupported method")
	})
	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}

type PoolRequest struct {
	Content string `json:"content"`
}
