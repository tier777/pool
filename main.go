package main

import (
	"fmt"
	"net/http"
)

func main() {
	db := InitDB()

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/pool/", PoolHandler(db))

	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
