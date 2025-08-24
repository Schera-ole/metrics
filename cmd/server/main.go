package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/repository"
)

func main() {
	address := flag.String("a", "localhost:8080", "address")
	flag.Parse()

	if envAddress := os.Getenv("ADDRESS"); envAddress != "" {
		*address = envAddress
	}
	storage := repository.NewMemStorage()
	log.Fatal(http.ListenAndServe(*address, handler.Router(storage)))
}
