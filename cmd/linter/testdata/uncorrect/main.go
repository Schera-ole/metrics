package main

import (
	"log"
	"os"
)

func main() {
	// Call function with error
	doSomething()
}

func doSomething() {
	// Call panic
	panic("Uncorrect") // want "found usage of panic"

	// Call log.Fatal not in main
	log.Fatal("Uncorrect") // want "found usage of log.Fatal outside of main function"

	// Call os.Exit not in main
	os.Exit(1) // want "found usage of os.Exit outside of main function"
}
