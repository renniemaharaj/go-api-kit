package files

import (
	"bytes"
	"errors"
	"io/fs"
)

type fileSnapshot struct {
	contents []byte
	info     fs.FileInfo
	etag     string
}

var errStreamFile = errors.New("stream file without caching")

type snapshotFile struct {
	*bytes.Reader
	info fs.FileInfo
}

func (f *snapshotFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

func (f *snapshotFile) Close() error {
	return nil
}

type snapshotInfo struct {
	fs.FileInfo
	size int64
}

func (i snapshotInfo) Size() int64 {
	return i.size
}
