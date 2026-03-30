package main

import (
	"fmt"
	"io"
	"net/http"
)

var pools = make(map[string]string)

func main() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/pool", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "code is required", http.StatusBadRequest)
			return
		}
		if r.Method == "POST" {
			body, error := io.ReadAll(r.Body)
			if error != nil {
				http.Error(w, "failed to read body", http.StatusInternalServerError)
				return
			}
			pools[code] = string(body)
			fmt.Fprint(w, "saved")
			return
		}
		if r.Method == "GET" {
			value, ok := pools[code]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			fmt.Fprintln(w, value)
			return
		}
		fmt.Fprint(w, "unsupported method")
	})
	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
