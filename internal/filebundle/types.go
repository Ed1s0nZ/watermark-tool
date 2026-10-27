// Package filebundle ingests bounded archives/folder manifests without using logical paths as filesystem paths.
package filebundle

import (
	"errors"
)

var (
	ErrArchive  = errors.New("bundle_archive_invalid")
	ErrPath     = errors.New("bundle_path_invalid")
	ErrLimit    = errors.New("bundle_limit")
	ErrLink     = errors.New("bundle_link_unsupported")
	ErrChecksum = errors.New("bundle_checksum")
	ErrIO       = errors.New("bundle_io")
	ErrEmpty    = errors.New("bundle_empty")
	ErrChanged  = errors.New("bundle_input_changed")
)

type Limits struct {
	MaxFiles         int
	MaxEntries       int
	MaxArchiveBytes  int64
	MaxFileBytes     int64
	MaxExpandedBytes int64
	MaxCentralBytes  int64
}

func DefaultLimits() Limits {
	return Limits{MaxFiles: 100, MaxEntries: 1024, MaxArchiveBytes: 128 << 20, MaxFileBytes: 50 << 20, MaxExpandedBytes: 128 << 20, MaxCentralBytes: 1 << 20}
}
func OfficeLimits() Limits {
	return Limits{MaxFiles: 4096, MaxEntries: 8192, MaxArchiveBytes: 50 << 20, MaxFileBytes: 50 << 20, MaxExpandedBytes: 128 << 20, MaxCentralBytes: 4 << 20}
}
func (l Limits) Valid() bool {
	return l.MaxFiles >= 1 && l.MaxFiles <= 4096 && l.MaxEntries >= l.MaxFiles && l.MaxEntries <= 8192 && l.MaxArchiveBytes >= 1 && l.MaxArchiveBytes <= 128<<20 && l.MaxFileBytes >= 1 && l.MaxFileBytes <= 50<<20 && l.MaxExpandedBytes >= 1 && l.MaxExpandedBytes <= 128<<20 && l.MaxCentralBytes >= 46 && l.MaxCentralBytes <= 8<<20
}

type Record struct {
	ID           string `json:"id"`
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
	Bytes        int64  `json:"bytes"`
	PhysicalPath string `json:"-"`
}
type Source struct{ RelativePath, PhysicalPath string }
