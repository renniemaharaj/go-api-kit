package kit

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (h Handle) Text(code int, text string) {
	h.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	h.w.WriteHeader(code)
	_, _ = io.WriteString(h.w, text)
}

func (h Handle) DistFile(path string) {
	path = strings.TrimPrefix(path, "/")
	path = filepath.Clean(path)

	if path == "." {
		path = "index.html"
	}

	if path == ".." || strings.HasPrefix(path, "../") {
		http.Error(h.w, "Invalid file path", http.StatusBadRequest)
		return
	}

	h.File(filepath.Join("dist", path))
}

func (h Handle) File(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(h.w, h.r)
			return
		}

		http.Error(h.w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(h.w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		http.NotFound(h.w, h.r)
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
