package v8

import (
	"io/fs"
	"net"
	"net/http"
	"os"
)

type rootFS struct {
	root *os.Root
}

func (f rootFS) Open(name string) (fs.File, error) {
	return f.root.Open(name)
}

func Run(dir, addr, port string) func() {
	return func() {
		root, err := os.OpenRoot(dir)
		if err != nil {
			panic(err)
		}
		defer root.Close()

		handler := http.FileServerFS(rootFS{root})
		err = http.ListenAndServe(net.JoinHostPort(addr, port), handler)
		if err != nil {
			panic(err)
		}
	}
}
