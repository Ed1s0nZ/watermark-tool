package filebundle

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"strings"
)

type archiveEntry struct {
	file      *zip.File
	name      string
	directory bool
}
type Archive struct {
	file        *os.File
	limits      Limits
	entries     []archiveEntry
	files       int
	directories []string
}

func Open(c context.Context, input string, limits Limits) (*Archive, error) {
	if !limits.Valid() {
		return nil, ErrLimit
	}
	info, err := os.Lstat(input)
	if err != nil {
		return nil, ErrIO
	}
	if !info.Mode().IsRegular() {
		return nil, ErrLink
	}
	f, err := os.Open(input)
	if err != nil {
		return nil, ErrIO
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	end, err := inspectCentral(c, f, info.Size(), limits)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(f, info.Size())
	if err != nil {
		if errors.Is(err, zip.ErrInsecurePath) {
			return nil, ErrPath
		}
		return nil, ErrArchive
	}
	if len(reader.File) != end.entries {
		return nil, ErrArchive
	}
	result := &Archive{file: f, limits: limits, entries: []archiveEntry{}, directories: []string{}}
	seen := newPathSet()
	declared := int64(0)
	for _, file := range reader.File {
		if err = c.Err(); err != nil {
			return nil, err
		}
		directory := file.FileInfo().IsDir()
		name := file.Name
		if directory {
			name = strings.TrimSuffix(name, "/")
		}
		if _, err = SafePath(name); err != nil {
			return nil, err
		}
		if file.NonUTF8 {
			for _, b := range []byte(name) {
				if b >= 128 {
					return nil, ErrPath
				}
			}
		}
		if kind := file.Mode() & os.ModeType; kind != 0 && kind != os.ModeDir {
			return nil, ErrLink
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return nil, ErrLink
		}
		if err = seen.add(name, directory); err != nil {
			return nil, err
		}
		if file.UncompressedSize64 > uint64(limits.MaxFileBytes) {
			return nil, ErrLimit
		}
		if directory && file.UncompressedSize64 != 0 {
			return nil, ErrArchive
		}
		if !directory {
			result.files++
			if result.files > limits.MaxFiles {
				return nil, ErrLimit
			}
			declared += int64(file.UncompressedSize64)
			if declared > limits.MaxExpandedBytes {
				return nil, ErrLimit
			}
		} else {
			result.directories = append(result.directories, name)
		}
		offset, err := file.DataOffset()
		if err != nil || offset < 0 || uint64(offset) > uint64(end.offset) || file.CompressedSize64 > uint64(end.offset)-uint64(offset) {
			return nil, ErrArchive
		}
		result.entries = append(result.entries, archiveEntry{file: file, name: name, directory: directory})
	}
	if result.files == 0 {
		return nil, ErrEmpty
	}
	ok = true
	return result, nil
}
func (a *Archive) Close() error          { return a.file.Close() }
func (a *Archive) Directories() []string { return append([]string{}, a.directories...) }
func archiveError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrLimit) || errors.Is(err, ErrIO) || errors.Is(err, ErrChanged) {
		return err
	}
	if errors.Is(err, zip.ErrChecksum) {
		return ErrChecksum
	}
	return ErrArchive
}
func (a *Archive) Validate(c context.Context) error {
	remaining := a.limits.MaxExpandedBytes
	for _, entry := range a.entries {
		reader, err := entry.file.Open()
		if err != nil {
			return archiveError(err)
		}
		max := a.limits.MaxFileBytes
		if entry.directory {
			max = 0
		}
		_, err = copyBounded(c, nil, reader, max, &remaining)
		reader.Close()
		if err != nil {
			return archiveError(err)
		}
	}
	return nil
}
