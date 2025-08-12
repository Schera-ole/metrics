package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Schera-ole/metrics/internal/handler"
)

func main() {
	address := flag.String("a", "localhost:8080", "address")
	flag.Parse()
	log.Fatal(http.ListenAndServe(*address, handler.Router()))
}
