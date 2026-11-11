package files

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/renniemaharaj/go-api-kit/internal/repository"
)

type fileRepository struct {
	fsys         fs.FS
	maxFileBytes int64
}

var errReadOnly = errors.New("file repository is read-only")

var _ repository.Store[fileSnapshot, string] = (*fileRepository)(nil)

func (r *fileRepository) Get(ctx context.Context, name string) (fileSnapshot, error) {
	var zero fileSnapshot
	file, err := r.open(ctx, name)
	if err != nil {
		return zero, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return zero, err
	}

	if !info.Mode().IsRegular() || info.Size() > r.maxFileBytes {
		return zero, errStreamFile
	}

	// The extra byte detects growth after Stat without caching a truncated file.
	contents, err := io.ReadAll(io.LimitReader(file, r.maxFileBytes+1))
	if err != nil {
		return zero, err
	}

	if int64(len(contents)) > r.maxFileBytes {
		return zero, errStreamFile
	}

	return fileSnapshot{
		contents: contents,
		info: snapshotInfo{
			FileInfo: info,
			size:     int64(len(contents)),
		},
		etag: fmt.Sprintf(`"%x"`, sha256.Sum256(contents)),
	}, nil
}

func (r *fileRepository) Save(context.Context, fileSnapshot) (fileSnapshot, error) {
	return fileSnapshot{}, errReadOnly
}

func (r *fileRepository) Delete(context.Context, string) error {
	return errReadOnly
}

func (r *fileRepository) open(ctx context.Context, name string) (fs.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	file, err := r.fsys.Open(name)
	if err != nil {
		return nil, err
	}

	return &contextFile{
		File: file,
		ctx:  ctx,
	}, nil
}
