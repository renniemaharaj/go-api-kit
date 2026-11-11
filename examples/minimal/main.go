// Run with: go run ./examples/minimal
package main

import (
	"log"
	"net/http"

	"github.com/renniemaharaj/go-api-kit/kit"
	"github.com/renniemaharaj/go-api-kit/v8"
)

func main() {
	if err := v8.Run(func(router *kit.Router) {
		router.Get("/hello", func(h kit.Handle) {
			h.Text(http.StatusOK, "Hello, world!")
		})
	}); err != nil {
		log.Fatal(err)
	}
}
