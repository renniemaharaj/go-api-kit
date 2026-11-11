// Run with: go run ./examples/resources
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/renniemaharaj/go-api-kit/examples/resources/users"
	"github.com/renniemaharaj/go-api-kit/kit"
	"github.com/renniemaharaj/go-api-kit/v8"
)

func main() {
	userID := flag.String("issue-token", "", "print a development JWT for this user and exit")
	flag.Parse()

	if *userID != "" {
		token, err := issueToken(*userID)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(token)
		return
	}

	resource, err := users.New(users.User{
		ID:   "42",
		Name: "Ada",
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := v8.Run(func(router *kit.Router) {
		resource.Routes(router, kit.RequireSubject("id"))
	}); err != nil {
		log.Fatal(err)
	}
}
