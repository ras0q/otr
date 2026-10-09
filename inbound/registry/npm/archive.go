package npm

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"time"

	"github.com/mholt/archives"
)

var npmPackFormat = archives.CompressedArchive{
	Compression: archives.Gz{},
	Archival:    archives.Tar{},
}

func writeNPMPack(ctx context.Context, writer io.Writer, files []archives.FileInfo) error {
	return npmPackFormat.Archive(ctx, writer, files)
}

func archiveBytes(nameInArchive string, mode fs.FileMode, data []byte) archives.FileInfo {
	info := fileInfo{
		name:    nameInArchive,
		size:    int64(len(data)),
		mode:    mode,
		modTime: time.Now(),
	}
	return archives.FileInfo{
		FileInfo:      info,
		NameInArchive: nameInArchive,
		Open: func() (fs.File, error) {
			return &readerFile{Reader: bytes.NewReader(data), info: info}, nil
		},
	}
}

func archiveStream(nameInArchive string, mode fs.FileMode, size int64, open func() (io.ReadCloser, error)) archives.FileInfo {
	info := fileInfo{
		name:    nameInArchive,
		size:    size,
		mode:    mode,
		modTime: time.Now(),
	}
	return archives.FileInfo{
		FileInfo:      info,
		NameInArchive: nameInArchive,
		Open: func() (fs.File, error) {
			readCloser, err := open()
			if err != nil {
				return nil, err
			}
			return &readerFile{Reader: readCloser, info: info, close: readCloser.Close}, nil
		},
	}
}

type fileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (f fileInfo) Name() string       { return f.name }
func (f fileInfo) Size() int64        { return f.size }
func (f fileInfo) Mode() fs.FileMode  { return f.mode }
func (f fileInfo) ModTime() time.Time { return f.modTime }
func (f fileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fileInfo) Sys() any           { return nil }

type readerFile struct {
	io.Reader
	info  fileInfo
	close func() error
}

func (f *readerFile) Close() error {
	if f.close != nil {
		return f.close()
	}
	return nil
}

func (f *readerFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}
