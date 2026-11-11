package files

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
)

func (r *Store) load(ctx context.Context, name string) (fileSnapshot, error) {
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

func (r *Store) open(ctx context.Context, name string) (fs.File, error) {
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
