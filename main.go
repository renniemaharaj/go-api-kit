package main

import (
	"log"

	"github.com/renniemaharaj/go-api-kit/v8"
)

func main() {
	if err := v8.Run(); err != nil {
		log.Fatal(err)
	}
}
