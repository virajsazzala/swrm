package downloader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/virajsazzala/swrm/internal/torrent"
)

type fileWriter struct {
	files []openFile
}

type openFile struct {
	f      *os.File
	offset int64
	length int64
}

func newFileWriter(t *torrent.Torrent, outputDir string) (*fileWriter, error) {
	fw := &fileWriter{}
	multiFile := len(t.Files) > 0

	for _, fi := range t.FileList() {
		var fullPath string
		if multiFile {
			parts := append([]string{outputDir, t.Name}, fi.Path...)
			fullPath = filepath.Join(parts...)
		} else {
			fullPath = filepath.Join(outputDir, fi.Path[0])
		}

		if dir := filepath.Dir(fullPath); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fw.Close()
				return nil, fmt.Errorf("failed to create directory for %s: %w", fullPath, err)
			}
		}

		f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			fw.Close()
			return nil, fmt.Errorf("failed to open %s: %w", fullPath, err)
		}

		if err := f.Truncate(fi.Length); err != nil {
			f.Close()
			fw.Close()
			return nil, fmt.Errorf("failed to pre-allocate %s: %w", fullPath, err)
		}

		fw.files = append(fw.files, openFile{f: f, offset: fi.Offset, length: fi.Length})
	}

	return fw, nil
}

func (fw *fileWriter) WriteAt(data []byte, offset int64) error {
	remaining := data
	pos := offset

	for len(remaining) > 0 {
		of, localOffset, err := fw.locate(pos)
		if err != nil {
			return err
		}

		writeLen := int64(len(remaining))
		if maxLen := of.length - localOffset; writeLen > maxLen {
			writeLen = maxLen
		}

		if _, err := of.f.WriteAt(remaining[:writeLen], localOffset); err != nil {
			return fmt.Errorf("failed to write at offset %d: %w", pos, err)
		}

		remaining = remaining[writeLen:]
		pos += writeLen
	}

	return nil
}

func (fw *fileWriter) locate(pos int64) (openFile, int64, error) {
	for _, of := range fw.files {
		if pos >= of.offset && pos < of.offset+of.length {
			return of, pos - of.offset, nil
		}
	}
	return openFile{}, 0, fmt.Errorf("offset %d out of range", pos)
}

func (fw *fileWriter) Sync() error {
	for _, of := range fw.files {
		if err := of.f.Sync(); err != nil {
			return fmt.Errorf("failed to sync %s: %w", of.f.Name(), err)
		}
	}
	return nil
}

func (fw *fileWriter) Close() {
	for _, of := range fw.files {
		if of.f != nil {
			of.f.Close()
		}
	}
}
