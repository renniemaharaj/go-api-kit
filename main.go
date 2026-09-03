package main

import (
	"net/http"

	"github.com/renniemaharaj/go-handler-kit/kit"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", kit.NewHandler(func(h kit.Handle) {
		if h.Path() == "/" {
			h.DistFile("index.html")
			return
		}

		h.DistFile(h.Path())
	}))

	err := http.ListenAndServe("localhost:8000", mux)
	if err != nil {
		panic(err)
	}
}
