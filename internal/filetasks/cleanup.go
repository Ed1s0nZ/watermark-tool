package filetasks

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"watermark-tool/internal/filebundle"
)

func directoryBytes(root string) (int64, error) {
	var bytes int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return ErrStore
		}
		bytes += info.Size()
		return nil
	})
	return bytes, err
}
func (m *Manager) usageLocked() (int64, error) { return directoryBytes(m.root) }
func (m *Manager) recover() error {
	next := clone(m.data)
	changed := false
	for id, j := range next.Jobs {
		if j.Status == "running" || j.Status == "queued" {
			j.Status = "interrupted"
			j.Error = "server_restarted"
			j.ArchiveAvailable = false
			j.Revision++
			j.UpdatedAt = m.now()
			changed = true
		}
		for i := range j.Items {
			item := &j.Items[i]
			if _, err := filebundle.SafePath(item.Source.RelativePath); err != nil {
				return ErrStore
			}
			if _, err := filebundle.SafePath(item.OutputPath); err != nil {
				return ErrStore
			}
			if item.Status == "running" || j.Status == "interrupted" && item.Status == "pending" {
				item.Status = "interrupted"
				item.Error = "server_restarted"
				if n := len(item.Attempts); n > 0 && item.Attempts[n-1].Status == "running" {
					item.Attempts[n-1].Status = "interrupted"
					item.Attempts[n-1].Error = "server_restarted"
					item.Attempts[n-1].At = m.now()
				}
				changed = true
			}
			if item.Status != "completed" {
				if err := os.Remove(filepath.Join(m.path("outputs", id), item.Source.ID)); err != nil && !os.IsNotExist(err) {
					return ErrStore
				}
				if err := os.Remove(filepath.Join(m.path("outputs", id), item.Source.ID+".report.json")); err != nil && !os.IsNotExist(err) {
					return ErrStore
				}
			}
		}
		next.Jobs[id] = j
	}
	if changed {
		if err := m.commitLocked(next); err != nil {
			return err
		}
	}
	return m.reconcileLocked()
}

func (m *Manager) reconcileLocked() error {
	// Only well-known manager-owned roots are reconciled. Never follow symlinks.
	for _, kind := range []string{"inputs", "outputs"} {
		entries, err := os.ReadDir(m.path(kind, ""))
		if err != nil {
			return ErrStore
		}
		for _, entry := range entries {
			if !entry.IsDir() || !idPattern.MatchString(entry.Name()) {
				return ErrStore
			}
			if _, ok := m.data.Jobs[entry.Name()]; !ok && !m.preparing[entry.Name()] {
				if err = os.RemoveAll(m.path(kind, entry.Name())); err != nil {
					return ErrStore
				}
			}
		}
	}
	for id, j := range m.data.Jobs {
		if m.running[id] != nil {
			continue
		}
		if !j.ExpiresAt.IsZero() && !m.now().Before(j.ExpiresAt) {
			if err := os.RemoveAll(m.path("outputs", id)); err != nil {
				return ErrStore
			}
			continue
		}
		entries, err := os.ReadDir(m.path("outputs", id))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return ErrStore
		}
		allowed := map[string]bool{}
		for _, item := range j.Items {
			if item.Status == "completed" {
				allowed[item.Source.ID] = true
				allowed[item.Source.ID+".report.json"] = true
			}
		}
		for _, entry := range entries {
			if allowed[entry.Name()] {
				continue
			}
			if err = os.RemoveAll(filepath.Join(m.path("outputs", id), entry.Name())); err != nil {
				return ErrStore
			}
		}
	}
	rootEntries, err := os.ReadDir(m.root)
	if err != nil {
		return ErrStore
	}
	for _, entry := range rootEntries {
		if strings.HasPrefix(entry.Name(), ".file-txn-") {
			if err = os.Remove(filepath.Join(m.root, entry.Name())); err != nil {
				return ErrStore
			}
		}
	}
	entries, err := os.ReadDir(m.path("archives", ""))
	if err != nil {
		return ErrStore
	}
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".zip")
		job, known := m.data.Jobs[id]
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrStore
		}
		if len(m.running) == 0 && strings.HasPrefix(entry.Name(), ".filezip-") && strings.HasSuffix(entry.Name(), ".tmp") || idPattern.MatchString(id) && (!known || !job.ArchiveAvailable) {
			if err = os.Remove(m.path("archives", entry.Name())); err != nil {
				return ErrStore
			}
		}
	}
	return nil
}
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.poisoned {
		return ErrStore
	}
	now := m.now()
	needsCommit := false
	for id, j := range m.data.Jobs {
		if m.running[id] != nil || j.Status == "queued" || j.Status == "running" {
			continue
		}
		if j.InputsAvailable && !now.Before(j.InputExpiresAt) || j.ArchiveAvailable && !now.Before(j.ExpiresAt) {
			needsCommit = true
			break
		}
	}
	if !needsCommit {
		if err := m.reconcileLocked(); err != nil {
			m.cleanupFault = true
			return err
		}
		m.cleanupFault = false
		return nil
	}
	next := clone(m.data)
	changed := false
	for id, j := range next.Jobs {
		jobChanged := false
		if m.running[id] != nil || j.Status == "queued" || j.Status == "running" {
			continue
		}
		if j.InputsAvailable && !now.Before(j.InputExpiresAt) {
			if err := os.RemoveAll(m.path("inputs", id)); err != nil {
				m.cleanupFault = true
				return ErrStore
			}
			j.InputsAvailable = false
			jobChanged = true
			changed = true
		}
		if !j.ExpiresAt.IsZero() && !now.Before(j.ExpiresAt) {
			if err := os.Remove(m.path("archives", id+".zip")); err != nil && !os.IsNotExist(err) {
				m.cleanupFault = true
				return ErrStore
			}
			if err := os.RemoveAll(m.path("outputs", id)); err != nil {
				m.cleanupFault = true
				return ErrStore
			}
			if j.ArchiveAvailable {
				j.ArchiveAvailable = false
				jobChanged = true
				changed = true
			}
		}
		if jobChanged {
			j.Revision++
			j.UpdatedAt = now
		}
		next.Jobs[id] = j
	}
	if changed {
		if err := m.commitLocked(next); err != nil {
			m.cleanupFault = true
			return err
		}
	}
	if err := m.reconcileLocked(); err != nil {
		m.cleanupFault = true
		return err
	}
	m.cleanupFault = false
	return nil
}
func (m *Manager) Delete(id string, revision uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.getLocked(id)
	if err != nil {
		return err
	}
	if m.running[id] != nil || j.Status == "queued" || j.Status == "running" || j.Revision != revision {
		return ErrState
	}
	next := clone(m.data)
	delete(next.Jobs, id)
	if err = m.commitLocked(next); err != nil {
		return err
	}
	for _, kind := range []string{"inputs", "outputs"} {
		if err = os.RemoveAll(m.path(kind, id)); err != nil {
			m.cleanupFault = true
			return ErrStore
		}
	}
	if err = os.Remove(m.path("archives", id+".zip")); err != nil && !os.IsNotExist(err) {
		m.cleanupFault = true
		return ErrStore
	}
	return nil
}

type Stats struct {
	Jobs           int   `json:"jobs"`
	Queued         int   `json:"queued"`
	Running        int   `json:"running"`
	Preparing      int   `json:"preparing"`
	DiskBytes      int64 `json:"disk_bytes"`
	ReservedBytes  int64 `json:"reserved_bytes"`
	CleanupFault   bool  `json:"cleanup_fault"`
	DiskLimitBytes int64 `json:"disk_limit_bytes"`
}

func (m *Manager) Stats() (Stats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.poisoned {
		return Stats{}, ErrStore
	}
	s := Stats{Jobs: len(m.data.Jobs), Preparing: len(m.preparing), Running: len(m.running), ReservedBytes: m.reserved, CleanupFault: m.cleanupFault, DiskLimitBytes: MaxDiskBytes}
	for _, j := range m.data.Jobs {
		if j.Status == "queued" {
			s.Queued++
		}
	}
	n, err := m.usageLocked()
	if err != nil {
		return Stats{}, ErrStore
	}
	s.DiskBytes = n
	return s, nil
}
