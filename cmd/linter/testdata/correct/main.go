package main

import (
	"log"
	"os"
)

func main() {
	// Вызов log.Fatal в main
	log.Fatal("это допустимо")

	// Вызов exit в main
	os.Exit(1)
}
