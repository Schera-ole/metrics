package main

import (
	"log"
	"os"
)

func main() {
	// Вызов функции с нарушением
	doSomething()
}

func doSomething() {
	// Вызов panic
	panic("недопустимо") // want "found usage of panic"

	// Вызов log.Fatal вне main
	log.Fatal("недопустимо") // want "found usage of log.Fatal outside of main function"

	// Вызов os.Exit вне main
	os.Exit(1) // want "found usage of os.Exit outside of main function"
}
