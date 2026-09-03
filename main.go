package main

import (
	"net/http"

	"github.com/renniemaharaj/go-handler-kit/kit"
)

func main() {
	k := kit.New()

	k.Get("/*", func(h kit.Handle) {
		if h.Path() == "/" {
			h.DistFile("index.html")
			return
		}

		h.DistFile(h.Path())
	})

	err := http.ListenAndServe("localhost:8000", k)
	if err != nil {
		panic(err)
	}
}
