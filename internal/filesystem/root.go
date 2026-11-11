package filesystem

import (
	"io/fs"
	"os"
)

// Root adapts an os.Root to fs.FS without allowing paths to escape its directory.
type Root struct {
	root *os.Root
}

func OpenRoot(directory string) (*Root, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}

	return &Root{root: root}, nil
}

func (r *Root) Open(name string) (fs.File, error) {
	return r.root.Open(name)
}

func (r *Root) Close() error {
	return r.root.Close()
}
