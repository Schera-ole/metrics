package main

import (
	"log"
	"net/http"

	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
)

func main() {
	storage := repository.NewMemStorage()
	router := chi.NewRouter()
	router.Post("/update/{type}/{metric}/{value}", func(w http.ResponseWriter, r *http.Request) {
		handler.UpdateHandler(w, r, storage)
	})
	router.Get("/value/{type}/{name}", func(w http.ResponseWriter, r *http.Request) {
		handler.GetHandler(w, r, storage)
	})
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		handler.GetListHandler(w, r, storage)
	})
	log.Fatal(http.ListenAndServe(":8080", router))
}
