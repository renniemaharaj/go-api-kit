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

func Run(dir, addr, port string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	handler := http.FileServerFS(rootFS{root})
	return http.ListenAndServe(net.JoinHostPort(addr, port), handler)
}
