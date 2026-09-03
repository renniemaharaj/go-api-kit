package kit

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
)

func (h Handle) DistFile(path string) {
	root, err := filepath.Abs("dist")
	if err != nil {
		h.Error.Internal("Internal Server Error")
		return
	}

	path = filepath.Clean(filepath.FromSlash(path))
	if path == "." || path == string(filepath.Separator) {
		path = "index.html"
	}
	if filepath.IsAbs(path) {
		path = path[1:]
	}

	filename := filepath.Join(root, path)
	if !within(root, filename) {
		h.Error.BadRequest("Invalid file path")
		return
	}

	resolved, err := filepath.EvalSymlinks(filename)
	if err == nil && !within(root, resolved) {
		h.Error.BadRequest("Invalid file path")
		return
	}

	h.File(filename)
}

func within(root, filename string) bool {
	rel, err := filepath.Rel(root, filename)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) &&
		(len(rel) < 3 || rel[:3] != ".."+string(filepath.Separator))
}

func (h Handle) File(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			h.Error.NotFound("Not Found")
			return
		}

		h.Error.Internal("Internal Server Error")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		h.Error.Internal("Internal Server Error")
		return
	}

	if info.IsDir() {
		h.Error.NotFound("Not Found")
		return
	}

	http.ServeContent(
		h.w,
		h.r,
		info.Name(),
		info.ModTime(),
		file,
	)
}
