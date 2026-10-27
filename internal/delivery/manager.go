// Package delivery owns bounded background processing and temporary job artifacts.
package delivery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/quality"
)

var ErrExpired = errors.New("archive_expired")
var ErrArchive = errors.New("archive_unavailable")

type Options struct{ ArchiveTTL time.Duration }
type Manager struct {
	archiveLimit int64
	cleanupFault bool
	ttl          time.Duration
	store        *distribution.Store
	root         string
	slots        chan struct{}
	ctx          context.Context
	stop         context.CancelFunc
	done         chan struct{}
	wg           sync.WaitGroup
	once         sync.Once
	mu           sync.Mutex
	cancels      map[string]context.CancelFunc
	reserved     int64
	bad          map[string]bool
	now          func() time.Time
}

func privateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return distribution.ErrStore
	}
	return nil
}
func New(store *distribution.Store, slots chan struct{}, options ...Options) (*Manager, error) {
	if store == nil || cap(slots) < 1 {
		return nil, distribution.ErrConfig
	}
	ttl := time.Hour
	if len(options) > 0 && options[0].ArchiveTTL != 0 {
		ttl = options[0].ArchiveTTL
	}
	if ttl < time.Second || ttl > time.Hour {
		return nil, distribution.ErrConfig
	}
	root := store.RootDir()
	for _, name := range []string{"jobs-inputs", "artifacts"} {
		if err := privateDir(filepath.Join(root, name)); err != nil {
			return nil, err
		}
	}
	if err := store.RecoverJobs(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{archiveLimit: distribution.MaxArchiveBytes, ttl: ttl, store: store, root: root, slots: slots, ctx: ctx, stop: cancel, done: make(chan struct{}), cancels: map[string]context.CancelFunc{}, bad: map[string]bool{}, now: time.Now}
	if err := m.Cleanup(); err != nil {
		cancel()
		return nil, err
	}
	go m.loop()
	return m, nil
}
func (m *Manager) Close() { m.once.Do(func() { m.stop(); <-m.done; m.wg.Wait() }) }
func (m *Manager) loop() {
	defer close(m.done)
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	lastClean := m.now()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-tick.C:
			if m.now().Sub(lastClean) >= time.Second {
				err := m.Cleanup()
				m.mu.Lock()
				m.cleanupFault = err != nil
				m.mu.Unlock()
				lastClean = m.now()
			}
			jobs, err := m.store.PendingJobs()
			if err != nil {
				continue
			}
			for _, job := range jobs {
				if job.Status == "queued" {
					m.dispatch(job.ID)
				}
			}
		}
	}
}
func (m *Manager) dispatch(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx.Err() != nil || m.cancels[id] != nil {
		return
	}
	select {
	case m.slots <- struct{}{}:
	default:
		return
	}
	usage, err := m.usage()
	if err != nil {
		m.cleanupFault = true
	}
	if err != nil || usage+m.reserved+distribution.MaxArchiveBytes > distribution.MaxTaskDiskBytes {
		<-m.slots
		return
	}
	job, key, err := m.store.StartJob(id)
	if err != nil {
		<-m.slots
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancels[id] = cancel
	m.reserved += distribution.MaxArchiveBytes
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			cancel()
			m.mu.Lock()
			delete(m.cancels, id)
			m.reserved -= distribution.MaxArchiveBytes
			m.mu.Unlock()
			<-m.slots
		}()
		hash, size, err := m.archive(ctx, job, key)
		status, code := "completed", ""
		if err != nil {
			status = "failed"
			code = errorCode(err)
			if errors.Is(err, context.Canceled) {
				status = "cancelled"
				code = "cancelled"
				if m.ctx.Err() != nil {
					status = "interrupted"
					code = "interrupted"
				}
			}
		}
		_ = os.RemoveAll(m.inputDir(id))
		_, _ = m.store.FinishJob(id, status, code, hash, size, m.ttl)
	}()
}
func errorCode(err error) string {
	switch {
	case errors.Is(err, quality.ErrThreshold):
		return "quality_failed"
	case errors.Is(err, distribution.ErrLimit):
		return "platform_limit"
	case errors.Is(err, distribution.ErrConflict):
		return "input_conflict"
	default:
		return "generation_failed"
	}
}
func (m *Manager) Start(id, actor string, retry bool) (distribution.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cleanupFault {
		return distribution.Job{}, distribution.ErrStore
	}
	j, err := m.store.Job(id)
	if err != nil {
		return j, err
	}
	if j.Status == "queued" || j.Status == "processing" || j.Status == "cancelling" {
		return j, nil
	}
	for _, src := range j.Sources {
		if err = m.checkInput(j, src.ID, ""); err != nil {
			return j, err
		}
	}
	if j.Mode == "dual" {
		for _, c := range j.Copies {
			if err = m.checkInput(j, c.SourceID, c.ID); err != nil {
				return j, err
			}
		}
	}
	return m.store.QueueJob(id, actor, retry)
}
func (m *Manager) Cancel(id, actor string) (distribution.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.store.CancelJob(id, actor)
	if err != nil {
		return j, err
	}
	if cancel := m.cancels[id]; cancel != nil {
		cancel()
	}
	if j.Status == "cancelled" {
		_ = os.RemoveAll(m.inputDir(id))
	}
	return j, nil
}
func (m *Manager) View(id string) (distribution.Job, error) {
	j, err := m.store.Job(id)
	if err != nil {
		return j, err
	}
	j.DownloadState = "pending"
	if j.Generated {
		j.DownloadState = "ready"
		if !m.now().Before(j.ExpiresAt) {
			j.DownloadState = "expired"
		} else {
			m.mu.Lock()
			bad := m.bad[id]
			m.mu.Unlock()
			info, e := os.Lstat(m.archivePath(id))
			if e != nil || !info.Mode().IsRegular() || info.Size() != j.ArchiveBytes || bad {
				j.DownloadState = "unavailable"
			}
		}
	}
	return j, nil
}
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs, err := m.store.AllJobs()
	if err != nil {
		return err
	}
	known := map[string]distribution.Job{}
	for _, j := range jobs {
		known[j.ID] = j
	}
	for _, j := range jobs {
		if j.Status == "preparing" && m.now().Sub(j.CreatedAt) >= 15*time.Minute {
			if _, err = m.store.FailPreparing(j.ID, "upload_expired"); err != nil {
				return err
			}
			j.Status = "failed"
			known[j.ID] = j
		}
		if !pending(j.Status) {
			if distribution.HasJobInputs(j) && m.now().Sub(j.UpdatedAt) < 15*time.Minute {
				continue
			}
			if distribution.HasJobInputs(j) {
				if err = m.store.ClearJobInputs(j.ID); err != nil {
					return err
				}
			}
			if err = os.RemoveAll(m.inputDir(j.ID)); err != nil {
				return err
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(m.root, "artifacts"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(m.root, "artifacts", e.Name())
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return distribution.ErrStore
		}
		ext := filepath.Ext(e.Name())
		id := e.Name()[:len(e.Name())-len(ext)]
		if ext != ".zip" && ext != ".part" {
			return distribution.ErrStore
		}
		j, ok := known[id]
		if m.cancels[id] != nil {
			continue
		}
		if !ok || ext == ".part" || !j.Generated || !m.now().Before(j.ExpiresAt) {
			if err = os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}
func pending(status string) bool {
	return status == "preparing" || status == "queued" || status == "processing" || status == "cancelling"
}

func (m *Manager) Degraded() bool { m.mu.Lock(); defer m.mu.Unlock(); return m.cleanupFault }
