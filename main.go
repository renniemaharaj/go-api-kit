package main

import (
	"log"

	v8 "github.com/renniemaharaj/go-api-kit/v8"
)

func main() {
	if err := v8.Run("dist", "localhost", "9100"); err != nil {
		log.Fatal(err)
	}
}
