package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"go.uber.org/zap"

	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/repository"
)

func main() {
	address := flag.String("a", "localhost:8080", "address")
	flag.Parse()
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to initialize zap logger: ", err)
	}
	defer logger.Sync()
	log_sugar := logger.Sugar()
	if envAddress := os.Getenv("ADDRESS"); envAddress != "" {
		*address = envAddress
	}
	storage := repository.NewMemStorage()
	log_sugar.Infow(
		"Starting server",
		"address", address,
	)
	log_sugar.Fatal(http.ListenAndServe(*address, handler.Router(storage, log_sugar)))
}
