package filebundle

import (
	"context"
	"os"
)

type Bundle struct {
	Files       []Record `json:"files"`
	Directories []string `json:"directories"`
}

// Extract creates a fresh private staging directory and removes the entire directory on any failure.
func (a *Archive) Extract(c context.Context, destination string) (result Bundle, err error) {
	if err = os.Mkdir(destination, 0700); err != nil {
		return Bundle{}, ErrIO
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(destination)
		}
	}()
	result = Bundle{Files: []Record{}, Directories: a.Directories()}
	remaining := a.limits.MaxExpandedBytes
	for _, entry := range a.entries {
		if err = c.Err(); err != nil {
			return Bundle{}, err
		}
		reader, openErr := entry.file.Open()
		if openErr != nil {
			return Bundle{}, archiveError(openErr)
		}
		if entry.directory {
			_, err = copyBounded(c, nil, reader, 0, &remaining)
			reader.Close()
			if err != nil {
				return Bundle{}, archiveError(err)
			}
			continue
		}
		record, copyErr := copyRecord(c, destination, entry.name, reader, int64(entry.file.UncompressedSize64), a.limits.MaxFileBytes, &remaining)
		reader.Close()
		if copyErr != nil {
			return Bundle{}, archiveError(copyErr)
		}
		result.Files = append(result.Files, record)
	}
	committed = true
	return result, nil
}
func Ingest(c context.Context, sources []Source, destination string, limits Limits) (result Bundle, err error) {
	if !limits.Valid() || len(sources) > limits.MaxFiles {
		return Bundle{}, ErrLimit
	}
	if len(sources) == 0 {
		return Bundle{}, ErrEmpty
	}
	seen := newPathSet()
	sizes := make([]int64, len(sources))
	declared := int64(0)
	for index, source := range sources {
		if err = c.Err(); err != nil {
			return Bundle{}, err
		}
		if _, err = SafePath(source.RelativePath); err != nil {
			return Bundle{}, err
		}
		if err = seen.add(source.RelativePath, false); err != nil {
			return Bundle{}, err
		}
		info, statErr := os.Lstat(source.PhysicalPath)
		if statErr != nil {
			return Bundle{}, ErrIO
		}
		if !info.Mode().IsRegular() {
			return Bundle{}, ErrLink
		}
		if info.Size() > limits.MaxFileBytes {
			return Bundle{}, ErrLimit
		}
		sizes[index] = info.Size()
		declared += info.Size()
		if declared > limits.MaxExpandedBytes {
			return Bundle{}, ErrLimit
		}
	}
	if err = os.Mkdir(destination, 0700); err != nil {
		return Bundle{}, ErrIO
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(destination)
		}
	}()
	result = Bundle{Files: []Record{}, Directories: []string{}}
	remaining := limits.MaxExpandedBytes
	for index, source := range sources {
		f, openErr := os.Open(source.PhysicalPath)
		if openErr != nil {
			return Bundle{}, ErrIO
		}
		record, copyErr := copyRecord(c, destination, source.RelativePath, f, sizes[index], limits.MaxFileBytes, &remaining)
		f.Close()
		if copyErr != nil {
			return Bundle{}, copyErr
		}
		result.Files = append(result.Files, record)
	}
	committed = true
	return result, nil
}
