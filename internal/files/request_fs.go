package files

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
)

type requestFileSystem struct {
	store  *Store
	ctx    context.Context
	header http.Header
}

func (f requestFileSystem) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}

	if !f.store.cacheEnabled {
		return f.store.open(f.ctx, name)
	}

	snapshot, err := f.store.files.Get(f.ctx, name)
	if errors.Is(err, errStreamFile) {
		return f.store.open(f.ctx, name)
	}

	if err != nil {
		return nil, err
	}

	f.header.Set("ETag", snapshot.etag)
	return &snapshotFile{
		Reader: bytes.NewReader(snapshot.contents),
		info:   snapshot.info,
	}, nil
}
