package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/pool", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello world")
		fmt.Println(r)
	})
	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
