package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
)

func (m *Manager) inputDir(id string) string { return filepath.Join(m.root, "jobs-inputs", id) }
func (m *Manager) inputPath(id, sourceID, copyID string) string {
	name := sourceID + ".source"
	if copyID != "" {
		name = copyID + ".png"
	}
	return filepath.Join(m.inputDir(id), name)
}
func (m *Manager) archivePath(id string) string { return filepath.Join(m.root, "artifacts", id+".zip") }
func fileHash(path string) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", 0, distribution.ErrStore
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}
func (m *Manager) usage() (int64, error) {
	var total int64
	for _, folder := range []string{"jobs-inputs", "artifacts"} {
		err := filepath.WalkDir(filepath.Join(m.root, folder), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return distribution.ErrStore
			}
			if !d.IsDir() {
				info, e := d.Info()
				if e != nil {
					return e
				}
				if !info.Mode().IsRegular() {
					return distribution.ErrStore
				}
				total += info.Size()
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}
func (m *Manager) checkInput(j distribution.Job, sourceID, copyID string) error {
	expected := ""
	uploaded := false
	if copyID == "" {
		for _, src := range j.Sources {
			if src.ID == sourceID {
				expected = src.SHA256
				uploaded = src.Uploaded
			}
		}
	} else {
		for _, c := range j.Copies {
			if c.SourceID == sourceID && c.ID == copyID {
				expected = c.RenderedSHA256
				uploaded = c.Uploaded
			}
		}
	}
	if expected == "" || !uploaded {
		return distribution.ErrJobState
	}
	hash, _, err := privateHash(m.inputPath(j.ID, sourceID, copyID))
	if err != nil {
		return distribution.ErrJobState
	}
	if hash != expected {
		return distribution.ErrConflict
	}
	return nil
}
func (m *Manager) Upload(id, sourceID, copyID, path string, tokens ...string) (distribution.Job, error) {
	img, err := blind.Load(path)
	if err == nil {
		err = blind.Validate(img)
	}
	if err != nil {
		return distribution.Job{}, err
	}
	hash, size, err := fileHash(path)
	if err != nil {
		return distribution.Job{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.store.Job(id)
	if err != nil {
		return j, err
	}
	if len(tokens) > 0 && tokens[0] != j.UploadToken {
		return j, distribution.ErrConflict
	}
	if j.Status != "preparing" && j.Status != "failed" && j.Status != "interrupted" && j.Status != "cancelled" {
		return j, distribution.ErrJobState
	}
	storedBytes := int64(0)
	found := false
	for _, src := range j.Sources {
		if src.ID == sourceID {
			found = true
			if copyID == "" {
				storedBytes = src.Bytes
			}
			if src.Width != img.Bounds().Dx() || src.Height != img.Bounds().Dy() {
				return j, distribution.ErrConflict
			}
			if copyID == "" && src.SHA256 != hash {
				return j, distribution.ErrConflict
			}
		}
	}
	if !found {
		return j, distribution.ErrMissing
	}
	if copyID != "" {
		if j.Mode != "dual" {
			return j, distribution.ErrJobState
		}
		found = false
		for _, c := range j.Copies {
			if c.ID == copyID && c.SourceID == sourceID {
				found = true
				storedBytes = c.InputBytes
				if c.RenderedSHA256 != "" && c.RenderedSHA256 != hash {
					return j, distribution.ErrConflict
				}
			}
		}
		if !found {
			return j, distribution.ErrMissing
		}
	}
	target := m.inputPath(id, sourceID, copyID)
	if err = privateDir(m.inputDir(id)); err != nil {
		return j, err
	}
	oldBytes := int64(0)
	if info, e := os.Lstat(target); e == nil {
		if !info.Mode().IsRegular() {
			return j, distribution.ErrStore
		}
		oldBytes = info.Size()
	}
	if oldBytes == size {
		existing, _, e := privateHash(target)
		if e == nil && existing == hash {
			return m.store.JobInput(id, sourceID, copyID, hash, size, j.UploadToken)
		}
	}
	usage, err := m.usage()
	if err != nil {
		return j, err
	}
	reserve := m.reserved
	if reserve < 2*distribution.MaxArchiveBytes {
		reserve = 2 * distribution.MaxArchiveBytes
	}
	if usage+reserve+size > distribution.MaxTaskDiskBytes {
		return j, distribution.ErrLimit
	}
	var jobBytes, sourceBytes int64
	for _, src := range j.Sources {
		sourceBytes += src.Bytes
	}
	jobBytes = sourceBytes
	for _, c := range j.Copies {
		jobBytes += c.InputBytes
	}
	if jobBytes+size-storedBytes > distribution.MaxJobInputBytes || copyID == "" && sourceBytes+size-storedBytes > distribution.MaxSourceBytes {
		return j, distribution.ErrLimit
	}
	in, err := os.Open(path)
	if err != nil {
		return j, err
	}
	defer in.Close()
	out, err := os.CreateTemp(m.inputDir(id), ".upload-")
	if err != nil {
		return j, err
	}
	name := out.Name()
	defer os.Remove(name)
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, target)
	}
	if err != nil {
		return j, err
	}
	return m.store.JobInput(id, sourceID, copyID, hash, size, j.UploadToken)
}
func (m *Manager) OpenArchive(id string) (*os.File, distribution.Job, error) {
	j, err := m.store.Job(id)
	if err != nil {
		return nil, j, err
	}
	if err = m.checkAuthorization(j); err != nil {
		return nil, j, err
	}
	if !j.Generated {
		return nil, j, distribution.ErrJobState
	}
	if !m.now().Before(j.ExpiresAt) {
		return nil, j, ErrExpired
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	hash, size, err := privateHash(m.archivePath(id))
	if err != nil || hash != j.ArchiveSHA256 || size != j.ArchiveBytes {
		m.bad[id] = true
		return nil, j, ErrArchive
	}
	f, err := os.Open(m.archivePath(id))
	if err != nil {
		return nil, j, ErrArchive
	}
	return f, j, nil
}

func privateHash(path string) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		return "", 0, distribution.ErrStore
	}
	return fileHash(path)
}
