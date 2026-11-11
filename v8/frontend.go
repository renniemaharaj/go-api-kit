package v8

import (
	"io/fs"
	"net/http"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
)

func frontendDefinition(fsys fs.FS) (data.Definition, error) {
	files, err := kit.NewFileServerFromEnv(fsys)
	if err != nil {
		return data.Definition{}, err
	}

	serve := func(h kit.Handle) {
		files.ServeHTTP(h.Writer(), h.Request())
	}

	return data.Definition{
		Name: "frontend",
		Routes: []data.Route{
			{
				Method:  http.MethodGet,
				Path:    "/*",
				Handler: serve,
			},
			{
				Method:  http.MethodHead,
				Path:    "/*",
				Handler: serve,
			},
		},
	}, nil
}
