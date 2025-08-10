package main

import (
	"net/http"

	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/repository"
)

func main() {
	storage := repository.NewMemStorage()
	mux := http.NewServeMux()
	mux.HandleFunc("/update/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.UpdateHandler(w, r, storage)
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	err := http.ListenAndServe(`:8080`, mux)
	if err != nil {
		panic(err)
	}
}
