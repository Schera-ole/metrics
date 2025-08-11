package main

import (
	"log"
	"net/http"

	"github.com/Schera-ole/metrics/internal/handler"
)

func main() {
	log.Fatal(http.ListenAndServe(":8080", handler.Router()))
}
