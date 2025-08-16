package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/repository"
)

func main() {
	address := flag.String("a", "localhost:8080", "address")
	flag.Parse()
	storage := repository.NewMemStorage()
	log.Fatal(http.ListenAndServe(*address, handler.Router(storage)))
}
