package filetasks

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"watermark-tool/internal/filebundle"
)

type state struct {
	Version int            `json:"version"`
	Team    string         `json:"team"`
	Jobs    map[string]Job `json:"jobs"`
}

var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func newID() (string, error) {
	raw := make([]byte, 16)
	_, err := rand.Read(raw)
	return hex.EncodeToString(raw), err
}
func clone[T any](value T) T {
	raw, _ := json.Marshal(value)
	var result T
	_ = json.Unmarshal(raw, &result)
	return result
}
func privateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return ErrStore
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return ErrStore
	}
	return nil
}
func atomicWrite(path string, raw []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".file-txn-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func decode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		return ErrStore
	}
	return nil
}
func (m *Manager) openStore(key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return ErrStore
	}
	m.aead, err = cipher.NewGCM(block)
	if err != nil {
		return ErrStore
	}
	m.aad = []byte("watermark/filetasks/v1/" + m.options.Team)
	path := filepath.Join(m.root, "tasks-v1.enc")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		entries, e := os.ReadDir(m.root)
		if e != nil {
			return ErrStore
		}
		for _, entry := range entries {
			if entry.Name() != ".lock" {
				return ErrStore
			}
		}
		m.data = state{Version: 1, Team: m.options.Team, Jobs: map[string]Job{}}
		return m.commitLocked(m.data)
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > MaxStateBytes+64 {
		return ErrStore
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) < m.aead.NonceSize() {
		return ErrStore
	}
	plain, err := m.aead.Open(nil, raw[:m.aead.NonceSize()], raw[m.aead.NonceSize():], m.aad)
	if err != nil || decode(plain, &m.data) != nil || m.validState(m.data) != nil {
		return ErrStore
	}
	return nil
}
func (m *Manager) validState(d state) error {
	if d.Version != 1 || d.Team != m.options.Team || d.Jobs == nil || len(d.Jobs) > MaxJobs {
		return ErrStore
	}
	for id, j := range d.Jobs {
		if id != j.ID || !idPattern.MatchString(id) || j.Team != d.Team || j.Operator == "" || len(j.Operator) > 256 || j.KeyID == "" || j.CreatedAt.IsZero() || j.Revision == 0 || len(j.Items) < 1 || len(j.Items) > 100 {
			return ErrStore
		}
		switch j.Status {
		case "ready", "queued", "running", "completed", "partial", "failed", "cancelled", "interrupted":
		default:
			return ErrStore
		}
		seen := map[string]bool{}
		var total int64
		for _, item := range j.Items {
			if !idPattern.MatchString(item.Source.ID) || seen[item.Source.ID] || !digestPattern.MatchString(item.Source.SHA256) || item.Source.Bytes < 1 || item.Source.Bytes > 50<<20 || len(item.Attempts) > MaxAttempts {
				return ErrStore
			}
			seen[item.Source.ID] = true
			if _, err := filebundle.SafePath(item.Source.RelativePath); err != nil {
				return ErrStore
			}
			if _, err := filebundle.SafePath(item.OutputPath); err != nil {
				return ErrStore
			}
			total += item.Source.Bytes
			if total > 128<<20 || len(item.Error) > 100 || len(item.PlanError) > 100 {
				return ErrStore
			}
			for index, attempt := range item.Attempts {
				if attempt.Number != index+1 || len(attempt.Error) > 100 || attempt.At.IsZero() {
					return ErrStore
				}
			}
			switch item.Status {
			case "pending", "running", "completed", "failed", "cancelled", "interrupted":
			default:
				return ErrStore
			}
			if item.Status == "completed" && (item.Result == nil || !item.Result.FinalVerified || !digestPattern.MatchString(item.Result.OutputSHA256)) {
				return ErrStore
			}
			if item.Status == "completed" && (!digestPattern.MatchString(item.ReportSHA256) || item.ReportBytes < 1 || item.ReportBytes > MaxJobBytes) {
				return ErrStore
			}
		}
		if j.ArchiveAvailable && (!digestPattern.MatchString(j.ArchiveSHA256) || j.ArchiveBytes < 1 || j.ArchiveBytes > MaxArchiveBytes || j.ExpiresAt.IsZero()) {
			return ErrStore
		}
	}
	return nil
}
func (m *Manager) commitLocked(next state) error {
	if m.poisoned || m.closed {
		return ErrStore
	}
	if err := m.validState(next); err != nil {
		return err
	}
	plain, err := json.Marshal(next)
	if err != nil {
		return ErrStore
	}
	if len(plain) > MaxStateBytes {
		return ErrLimit
	}
	nonce := make([]byte, m.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return ErrStore
	}
	raw := m.aead.Seal(nonce, nonce, plain, m.aad)
	if err = m.write(filepath.Join(m.root, "tasks-v1.enc"), raw); err != nil {
		m.poisoned = true
		return ErrStore
	}
	m.data = next
	return nil
}
