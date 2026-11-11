package files

import (
	"context"
	"io"
	"io/fs"
)

type contextFile struct {
	fs.File
	ctx context.Context
}

func (f *contextFile) Read(buffer []byte) (int, error) {
	if err := f.ctx.Err(); err != nil {
		return 0, err
	}

	return f.File.Read(buffer)
}

func (f *contextFile) Stat() (fs.FileInfo, error) {
	if err := f.ctx.Err(); err != nil {
		return nil, err
	}

	return f.File.Stat()
}

func (f *contextFile) Seek(offset int64, whence int) (int64, error) {
	if err := f.ctx.Err(); err != nil {
		return 0, err
	}

	seeker, ok := f.File.(io.Seeker)
	if !ok {
		return 0, fs.ErrInvalid
	}

	return seeker.Seek(offset, whence)
}

func (f *contextFile) ReadDir(count int) ([]fs.DirEntry, error) {
	if err := f.ctx.Err(); err != nil {
		return nil, err
	}

	directory, ok := f.File.(fs.ReadDirFile)
	if !ok {
		return nil, fs.ErrInvalid
	}

	return directory.ReadDir(count)
}
