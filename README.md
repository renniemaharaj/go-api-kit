# go-handler-kit

A small Go package that provides a cleaner experience when working with HTTP handlers.

It wraps the usual `http.ResponseWriter` and `*http.Request` constructs so you don't have to constantly repeat them and clutter your handlers.

## Installation

```bash
go get github.com/renniemaharaj/go-handler-kit

mux.HandleFunc("/", kit.NewHandler(func(h kit.Handle) {
	if h.Path() == "/" {
		h.DistFile("index.html")
		return
	}

	h.DistFile(h.Path())
}))
```

Built on top of Go's standard net/http package. Hope you like the api <3