package main

import (
	"fmt"
	"net/http"
)

var pools = make(map[string]string)

func main() {
	http.HandleFunc("/pool", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "code is required", http.StatusBadRequest)
			return
		}
		if r.Method == "POST" {
			body := make([]byte, r.ContentLength)
			r.Body.Read(body)
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
