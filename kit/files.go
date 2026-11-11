package kit

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

func (h Handle) StaticFS(fsys fs.FS, path string) {
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		path = "index.html"
	}

	if !fs.ValidPath(path) {
		h.Error.BadRequest("Invalid file path")
		return
	}

	file, err := fsys.Open(path)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrPermission):
			h.Error.BadRequest("Invalid file path")
		case errors.Is(err, fs.ErrNotExist):
			h.Error.NotFound("Not Found")
		default:
			h.Error.Internal("Internal Server Error")
		}

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

	request := h.Request().Clone(h.Request().Context())
	request.URL.Path = "/" + path

	if path == "index.html" {
		request.URL.Path = "/"
	}

	http.FileServerFS(fsys).ServeHTTP(h.Writer(), request)
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
