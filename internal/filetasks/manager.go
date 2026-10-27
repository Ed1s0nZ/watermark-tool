package filetasks

import (
	"context"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/fileprocess"
)

type Manager struct {
	mu                                      sync.Mutex
	options                                 Options
	root                                    string
	lock                                    *os.File
	aead                                    cipher.AEAD
	aad                                     []byte
	data                                    state
	poisoned, closed, closing, cleanupFault bool
	write                                   func(string, []byte) error
	ctx                                     context.Context
	stop                                    context.CancelFunc
	done                                    chan struct{}
	wg                                      sync.WaitGroup
	once                                    sync.Once
	running                                 map[string]context.CancelFunc
	preparing                               map[string]bool
	reserved                                int64
}

func New(root string, options Options) (*Manager, error) {
	if options.Team == "" || len(options.Team) > 64 || len(options.MetadataKey) != 32 || cap(options.Slots) < 1 || options.CurrentKey == nil || options.ResolveKey == nil {
		return nil, ErrStore
	}
	if options.ArchiveTTL == 0 {
		options.ArchiveTTL = time.Hour
	}
	if options.ArchiveTTL < time.Second || options.ArchiveTTL > time.Hour {
		return nil, ErrStore
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if err := privateDir(root); err != nil {
		return nil, err
	}
	lock, err := lockFile(filepath.Join(root, ".lock"))
	if err != nil {
		return nil, ErrStore
	}
	ctx, stop := context.WithCancel(context.Background())
	m := &Manager{options: options, root: root, lock: lock, ctx: ctx, stop: stop, done: make(chan struct{}), write: atomicWrite, running: map[string]context.CancelFunc{}, preparing: map[string]bool{}}
	ok := false
	defer func() {
		if !ok {
			stop()
			lock.Close()
		}
	}()
	if err = m.openStore(options.MetadataKey); err != nil {
		return nil, err
	}
	m.options.MetadataKey = nil
	for _, dir := range []string{"inputs", "outputs", "archives"} {
		if err = privateDir(filepath.Join(root, dir)); err != nil {
			return nil, err
		}
	}
	if err = m.recover(); err != nil {
		return nil, err
	}
	if err = m.Cleanup(); err != nil {
		return nil, err
	}
	ok = true
	go m.loop()
	return m, nil
}
func (m *Manager) now() time.Time { return m.options.Now().UTC() }
func (m *Manager) Close() {
	m.once.Do(func() {
		m.mu.Lock()
		m.closing = true
		m.stop()
		m.mu.Unlock()
		<-m.done
		m.wg.Wait()
		m.mu.Lock()
		m.closed = true
		m.lock.Close()
		m.mu.Unlock()
	})
}
func (m *Manager) path(kind, id string) string { return filepath.Join(m.root, kind, id) }
func (m *Manager) source(job string, source filebundle.Record) filebundle.Record {
	source.PhysicalPath = filepath.Join(m.path("inputs", job), source.ID)
	return source
}
func (m *Manager) saveJobLocked(j Job) error {
	next := clone(m.data)
	j.Revision++
	j.UpdatedAt = m.now()
	next.Jobs[j.ID] = j
	return m.commitLocked(next)
}
func (m *Manager) getLocked(id string) (Job, error) {
	if m.closed || m.poisoned {
		return Job{}, ErrStore
	}
	j, ok := m.data.Jobs[id]
	if !ok {
		return Job{}, ErrMissing
	}
	return clone(j), nil
}
func (m *Manager) Get(id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getLocked(id)
}
func (m *Manager) List() ([]Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.poisoned {
		return nil, ErrStore
	}
	result := []Job{}
	for _, j := range m.data.Jobs {
		result = append(result, clone(j))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func (m *Manager) gate(j Job) ([]byte, error) {
	if m.options.Authorize != nil {
		if err := m.options.Authorize(clone(j)); err != nil {
			return nil, err
		}
	}
	key, err := m.options.ResolveKey(j.KeyID)
	if err != nil || len(key) != 32 {
		return nil, ErrDenied
	}
	return key, nil
}

// Create ingests and parses original copies, then atomically fixes a ready job.
// Per-file format errors are recorded; archive/path errors reject the whole ingest.
func (m *Manager) Create(ctx context.Context, actor string, request Request, ingest Ingester) (Job, error) {
	if actor == "" || len(actor) > 256 || ingest == nil {
		return Job{}, ErrState
	}
	request = clone(request)
	raw, err := json.Marshal(request)
	if err != nil || len(raw) > MaxJobBytes/2 || len(request.PerFile) > 100 {
		return Job{}, ErrLimit
	}
	id, err := newID()
	if err != nil {
		return Job{}, ErrStore
	}
	m.mu.Lock()
	if m.closing || m.poisoned || len(m.data.Jobs)+len(m.preparing) >= MaxJobs {
		m.mu.Unlock()
		return Job{}, ErrLimit
	}
	usage, err := m.usageLocked()
	if err != nil || usage+m.reserved+128<<20 > MaxDiskBytes {
		m.mu.Unlock()
		return Job{}, ErrLimit
	}
	m.preparing[id] = true
	m.reserved += 128 << 20
	m.wg.Add(1)
	m.mu.Unlock()
	defer m.wg.Done()
	committed := false
	defer func() {
		m.mu.Lock()
		delete(m.preparing, id)
		m.reserved -= 128 << 20
		if !committed {
			os.RemoveAll(m.path("inputs", id))
		}
		m.mu.Unlock()
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	undo := context.AfterFunc(m.ctx, cancel)
	defer undo()
	select {
	case m.options.Slots <- struct{}{}:
		defer func() { <-m.options.Slots }()
	default:
		return Job{}, ErrBusy
	}
	keyID, key, err := m.options.CurrentKey()
	if err != nil || len(key) != 32 {
		return Job{}, ErrDenied
	}
	clear(key)
	bundle, err := ingest(ctx, m.path("inputs", id))
	if err != nil {
		return Job{}, err
	}
	if len(bundle.Files) < 1 || len(bundle.Files) > 100 || len(bundle.Directories) > 1024 {
		return Job{}, ErrLimit
	}
	now := m.now().Truncate(time.Second)
	j := Job{ID: id, Team: m.options.Team, Operator: actor, KeyID: keyID, CreatedAt: now, UpdatedAt: now, InputExpiresAt: now.Add(InputTTL), Status: "ready", InputsAvailable: true, Items: []Item{}, Directories: bundle.Directories}
	initial, _ := json.Marshal(j)
	estimated := len(initial)
	var totalInput int64
	plans := map[string]fileprocess.Plan{}
	known := map[string]bool{}
	for _, source := range bundle.Files {
		totalInput += source.Bytes
		if totalInput > 128<<20 {
			return Job{}, ErrLimit
		}
		if err = ctx.Err(); err != nil {
			return Job{}, err
		}
		// A trusted ingester must have placed random IDs under our own directory.
		if !idPattern.MatchString(source.ID) || source.PhysicalPath != m.source(id, source).PhysicalPath {
			return Job{}, ErrState
		}
		known[source.RelativePath] = true
		options := request.Options
		if specific, ok := request.PerFile[source.RelativePath]; ok {
			options = specific
		}
		item := Item{Source: source, Options: options, Status: "pending", Attempts: []Attempt{}}
		identity := fileprocess.Identity{DocumentID: source.ID, KeyID: keyID, CreatedAt: now}
		plan, e := fileprocess.Preview(ctx, source, identity, options)
		if e != nil {
			if ctx.Err() != nil {
				return Job{}, ctx.Err()
			}
			item.PlanError = errorCode(e)
		} else {
			item.Plan = &plan
			plans[source.ID] = plan
		}
		encoded, e := json.Marshal(item)
		if e != nil {
			return Job{}, ErrState
		}
		estimated += len(encoded) + 1
		if estimated > MaxJobBytes {
			return Job{}, ErrLimit
		}
		j.Items = append(j.Items, item)
	}
	for name := range request.PerFile {
		if !known[name] {
			return Job{}, ErrState
		}
	}
	names, err := fileprocess.OutputPathsWithDirectories(bundle.Files, plans, bundle.Directories)
	if err != nil {
		return Job{}, err
	}
	for i := range j.Items {
		j.Items[i].OutputPath = names[j.Items[i].Source.ID]
	}
	raw, err = json.Marshal(j)
	if err != nil || len(raw) > MaxJobBytes {
		return Job{}, ErrLimit
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing || ctx.Err() != nil {
		return Job{}, context.Canceled
	}
	next := clone(m.data)
	j.Revision = 1
	next.Jobs[id] = j
	// Reserve 6KiB per item for bounded attempts and compact final results.
	plain, _ := json.Marshal(next)
	reserve := 0
	for _, job := range next.Jobs {
		reserve += len(job.Items) * 6144
	}
	if len(plain)+reserve > MaxStateBytes-(1<<20) {
		return Job{}, ErrLimit
	}
	if err = m.commitLocked(next); err != nil {
		return Job{}, err
	}
	committed = true
	return clone(j), nil
}

func (m *Manager) Start(id string, revision uint64) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.getLocked(id)
	if err != nil {
		return Job{}, err
	}
	if m.closing || j.Revision != revision || m.running[id] != nil || j.Status == "running" || j.Status == "queued" || (j.Status == "completed" && j.ArchiveAvailable) {
		return Job{}, ErrState
	}
	if !j.ExpiresAt.IsZero() && !m.now().Before(j.ExpiresAt) {
		return Job{}, ErrExpired
	}
	if !j.InputsAvailable || !m.now().Before(j.InputExpiresAt) {
		return Job{}, ErrExpired
	}
	if _, err = m.gate(j); err != nil {
		return Job{}, err
	}
	count := 0
	for i := range j.Items {
		item := &j.Items[i]
		if item.Status != "completed" && len(item.Attempts) < MaxAttempts {
			item.Status = "pending"
			item.Error = ""
			count++
		}
	}
	if count == 0 && (j.Status != "completed" || j.ArchiveAvailable) {
		return Job{}, ErrState
	}
	j.Status = "queued"
	j.QueuedAt = m.now()
	j.FinishedAt = time.Time{}
	j.Error = ""
	j.ArchiveAvailable = false
	if err = m.saveJobLocked(j); err != nil {
		return Job{}, err
	}
	return m.getLocked(id)
}
func (m *Manager) Cancel(id string, revision uint64) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.getLocked(id)
	if err != nil {
		return Job{}, err
	}
	if j.Revision != revision {
		return Job{}, ErrState
	}
	if j.Status != "ready" && j.Status != "queued" && j.Status != "running" {
		return Job{}, ErrState
	}
	if cancel := m.running[id]; cancel != nil {
		cancel()
		j.Error = "cancel_requested"
	} else {
		j.Status = "cancelled"
		for i := range j.Items {
			if j.Items[i].Status == "pending" {
				j.Items[i].Status = "cancelled"
			}
		}
	}
	if err = m.saveJobLocked(j); err != nil {
		return Job{}, err
	}
	return m.getLocked(id)
}
func (m *Manager) loop() {
	defer close(m.done)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	lastClean := m.now()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-tick.C:
			if m.now().Sub(lastClean) >= time.Second {
				_ = m.Cleanup()
				lastClean = m.now()
			}
			m.mu.Lock()
			queued := []string{}
			if !m.poisoned && !m.closed {
				for id, job := range m.data.Jobs {
					if job.Status == "queued" {
						queued = append(queued, id)
					}
				}
			}
			m.mu.Unlock()
			for _, id := range queued {
				m.dispatch(id)
			}
		}
	}
}

func terminal(status string) bool {
	return status != "ready" && status != "queued" && status != "running"
}
func cancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
