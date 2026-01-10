package main

import (
	"log"
	"os"
)

func main() {
	// Call log.Fatal in main
	log.Fatal("it is okay")

	// Call exit in main
	os.Exit(1)
}
