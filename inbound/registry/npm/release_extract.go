package npm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/mholt/archives"
)

type releaseFile struct {
	path string
	data []byte
}

func binaryFromReleaseArchive(ctx context.Context, archiveName string, archive io.Reader) (fileName string, data []byte, err error) {
	raw, err := io.ReadAll(archive)
	if err != nil {
		return "", nil, fmt.Errorf("read release archive: %w", err)
	}

	format, reader, err := archives.Identify(ctx, archiveName, bytes.NewReader(raw))
	if err != nil {
		return "", nil, fmt.Errorf("identify release archive: %w", err)
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return "", nil, fmt.Errorf("unsupported release archive format for %q", archiveName)
	}

	var files []releaseFile
	err = extractor.Extract(ctx, reader, func(_ context.Context, info archives.FileInfo) error {
		if info.IsDir() {
			return nil
		}
		rc, err := info.Open()
		if err != nil {
			return err
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return err
		}
		files = append(files, releaseFile{
			path: path.Clean(info.NameInArchive),
			data: body,
		})
		return nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("extract release archive: %w", err)
	}

	picked := pickReleaseBinary(files)
	if picked == nil {
		return "", nil, fmt.Errorf("no binary found in release archive %q", archiveName)
	}
	return path.Base(picked.path), picked.data, nil
}

func pickReleaseBinary(files []releaseFile) *releaseFile {
	var binDir []releaseFile
	for i := range files {
		normalized := strings.ToLower(files[i].path)
		if strings.Contains(normalized, "/bin/") {
			base := path.Base(normalized)
			if base == "" || strings.HasSuffix(base, ".1") {
				continue
			}
			binDir = append(binDir, files[i])
		}
	}
	if len(binDir) == 1 {
		return &binDir[0]
	}
	if len(binDir) > 1 {
		best := &binDir[0]
		for i := range binDir[1:] {
			if len(binDir[i+1].data) > len(best.data) {
				best = &binDir[i+1]
			}
		}
		return best
	}
	return nil
}
