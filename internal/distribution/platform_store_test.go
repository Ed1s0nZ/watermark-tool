package distribution

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"watermark-tool/internal/blind"
)

func legacyFixture(t *testing.T) (string, Batch) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "legacy")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	old := &Store{dir: dir, master: testMaster, busy: map[string]bool{}, write: atomicWrite}
	now := time.Now().UTC().Truncate(time.Second)
	if err := old.saveKeys(keyState{1, []keySecret{{Key: Key{ID: "legacy-env", State: "active", CreatedAt: now, UpdatedAt: now, CreatedBy: "bootstrap"}, Secret: testLegacy}}}); err != nil {
		t.Fatal(err)
	}
	if err := old.saveState(state{Version: 1, Batches: []batchDisk{}}); err != nil {
		t.Fatal(err)
	}
	b, _, err := old.Begin("legacy-fixture-request", strings.Repeat("a", 64), "admin", recipients)
	if err != nil {
		t.Fatal(err)
	}
	if err = old.Finish(b.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "format.json"), []byte(`{"version":1,"ready":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	return dir, b
}
func TestV1MigrationAndAtomicAuditedKeyMutation(t *testing.T) {
	dir, b := legacyFixture(t)
	before, _ := os.ReadFile(filepath.Join(dir, "records.json"))
	s, err := Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	rows, total, err := s.Batches("", 1, 20)
	if err != nil || total != 1 || rows[0].ID != b.ID || !rows[0].Generated {
		t.Fatal(rows, err)
	}
	old, _ := os.ReadFile(filepath.Join(dir, "records.json"))
	if !bytes.Equal(old, before) {
		t.Fatal("legacy evidence changed")
	}
	events, _, _ := s.AuditSearch(AuditFilter{Page: 1, Size: 20}, false)
	if len(events) != 1 || events[0].Action != "storage.migrate" {
		t.Fatal(events)
	}
	key, err := s.CreateKey("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetKeyState(key.ID, "active", "operator"); err != nil {
		t.Fatal(err)
	}
	events, _, _ = s.AuditSearch(AuditFilter{Action: "key.state", Page: 1, Size: 20}, false)
	if len(events) != 1 || events[0].Actor != "operator" || events[0].Target != key.ID {
		t.Fatal(events)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "platform.json"))
	if bytes.Contains(raw, testLegacy) {
		t.Fatal("plaintext key")
	}
	s.Close()
	s, err = Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, _, _ := s.Current()
	if id != key.ID {
		t.Fatal("key lost")
	}
	mark, e := blind.Embed(loadFixture(t), b.Copies[0].ID, testLegacy, b.CreatedAt)
	if e != nil {
		t.Fatal(e)
	}
	result, verifiedKey, e := s.Verify(mark, "")
	if e != nil {
		t.Fatal(e)
	}
	record, e := s.Lookup(result, verifiedKey)
	if e != nil || record == nil || record.BatchID != b.ID {
		t.Fatal(record, e)
	}
}
func TestAuditAndBusinessCommitTogetherOnUncertainWrite(t *testing.T) {
	for _, afterRename := range []bool{false, true} {
		t.Run(map[bool]string{false: "before", true: "after"}[afterRename], func(t *testing.T) {
			s, dir := openTest(t)
			oldKeys, _ := s.Keys()
			oldEvents, total, _ := s.AuditSearch(AuditFilter{Page: 1, Size: 100}, false)
			_ = oldEvents
			s.write = func(path string, data []byte) error {
				if afterRename {
					if err := atomicWrite(path, data); err != nil {
						return err
					}
				}
				return os.ErrPermission
			}
			if _, err := s.CreateKey("admin"); !errors.Is(err, ErrStore) {
				t.Fatal(err)
			}
			if !s.AuditDegraded() {
				t.Fatal("uncertain state not degraded")
			}
			s.Close()
			next, err := Open(dir, testMaster, testLegacy)
			if err != nil {
				t.Fatal(err)
			}
			defer next.Close()
			keys, _ := next.Keys()
			events, newTotal, _ := next.AuditSearch(AuditFilter{Page: 1, Size: 100}, false)
			expected := 0
			if afterRename {
				expected = 1
			}
			if len(keys) != len(oldKeys)+expected || newTotal != total+expected {
				t.Fatal(len(keys), newTotal)
			}
			if afterRename && events[0].Action != "key.create" {
				t.Fatal(events)
			}
		})
	}
}
func TestJobsIdentityLifecycleAndRegisteredMatrix(t *testing.T) {
	s, dir := openTest(t)
	source := []JobSource{{Name: "first.png", SHA256: Digest([]byte("first")), Width: 512, Height: 512}, {Name: "second.png", SHA256: Digest([]byte("second")), Width: 512, Height: 512}}
	var wg sync.WaitGroup
	jobIDs := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, e := s.NewJob("matrix-request-001", "admin", source, recipients, "blind", "", nil)
			if e != nil {
				t.Error(e)
				return
			}
			jobIDs <- j.ID
		}()
	}
	wg.Wait()
	close(jobIDs)
	id := ""
	for v := range jobIDs {
		if id != "" && id != v {
			t.Fatal("duplicate task")
		}
		id = v
	}
	j, err := s.Job(id)
	if err != nil || len(j.Copies) != 6 || j.RequestHash != "" || j.Fingerprint != "" {
		t.Fatal(j, err)
	}
	if _, err = s.QueueJob(id, "admin", false); !errors.Is(err, ErrJobState) {
		t.Fatal("queued without inputs", err)
	}
	for _, src := range j.Sources {
		if _, err = s.JobInput(id, src.ID, "", src.SHA256, 100); err != nil {
			t.Fatal(err)
		}
	}
	changed := append([]JobSource(nil), source...)
	changed[0].SHA256 = Digest([]byte("changed"))
	if _, err = s.NewJob("matrix-request-001", "admin", changed, recipients, "blind", "", nil); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err = s.QueueJob(id, "admin", false); err != nil {
		t.Fatal(err)
	}
	next, err := s.CreateKey("admin")
	if err != nil {
		t.Fatal(err)
	}
	s.SetKeyState(next.ID, "active", "admin")
	running, key, err := s.StartJob(id)
	if err != nil || running.KeyID != "legacy-env" || !bytes.Equal(key, testLegacy) {
		t.Fatal(running, err)
	}
	before, _, _ := s.Records("", 1, 20)
	if len(before) != 6 || before[0].Generated {
		t.Fatal(before)
	}
	if err = s.JobProgress(id, 6, "packing"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.FinishJob(id, "completed", "", Digest([]byte("zip")), 100); err != nil {
		t.Fatal(err)
	}
	for _, copy := range j.Copies {
		marked, e := blind.Embed(loadFixture(t), copy.ID, key, j.CreatedAt)
		if e != nil {
			t.Fatal(e)
		}
		result, version, e := s.Verify(marked, "")
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.Lookup(result, version)
		if e != nil || r == nil || r.JobID != id || r.SourceID != copy.SourceID || r.Recipient.ID != copy.Recipient.ID {
			t.Fatal(r, e)
		}
	}
	s.Close()
	reopened, err := Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := reopened.Job(id)
	if err != nil || !persisted.Generated || persisted.ArchiveSHA256 != Digest([]byte("zip")) {
		t.Fatal(persisted, err)
	}
}
func TestJobInputSnapshotsAndRecovery(t *testing.T) {
	s, dir := openTest(t)
	style := map[string]any{"mode": "single", "position": "center", "size": float64(28), "opacity": 0.35, "color": "#365bdb", "angle": float64(-30)}
	srcs := []JobSource{{Name: "image.png", SHA256: Digest([]byte("original")), Width: 512, Height: 512}}
	j, err := s.NewJob("dual-snapshot-001", "admin", srcs, recipients, "dual", "内部 {name} · {recipient_id}", style)
	if err != nil {
		t.Fatal(err)
	}
	style["color"] = "#ff0000"
	persisted, _ := s.Job(j.ID)
	if persisted.Style["color"] != "#365bdb" {
		t.Fatal("style alias")
	}
	for _, c := range j.Copies {
		if _, err = s.JobInput(j.ID, c.SourceID, c.ID, Digest([]byte(c.ID)), 100); err != nil {
			t.Fatal(err)
		}
	}
	s.JobInput(j.ID, j.Sources[0].ID, "", srcs[0].SHA256, 100)
	s.QueueJob(j.ID, "admin", false)
	s.StartJob(j.ID)
	s.Close()
	reopened, err := Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err = reopened.RecoverJobs(); err != nil {
		t.Fatal(err)
	}
	recovered, _ := reopened.Job(j.ID)
	if recovered.Status != "interrupted" || recovered.Copies[0].Uploaded || recovered.Sources[0].Uploaded {
		t.Fatal(recovered)
	}
	if _, err = reopened.JobInput(j.ID, j.Sources[0].ID, "", srcs[0].SHA256, 100, j.UploadToken); !errors.Is(err, ErrConflict) {
		t.Fatal("stale upload token accepted", err)
	}

	if _, err = reopened.JobInput(j.ID, j.Copies[0].SourceID, j.Copies[0].ID, Digest([]byte("different render")), 100); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err = VisibleText("{unknown}", recipients[0]); err == nil {
		t.Fatal("unknown placeholder")
	}
	if _, err = reopened.NewJob("bad-style-0000001", "admin", srcs, recipients, "dual", "{name}", map[string]any{}); err == nil {
		t.Fatal("invalid style")
	}
}

func TestCSVHiddenFormulaPrefix(t *testing.T) {
	for _, value := range []string{"=SUM(1,2)", "\ufeff=SUM(1,2)", "\u00a0@formula", "\t-1"} {
		if CSVCell(value) != "'"+value {
			t.Fatal(value)
		}
	}
	if CSVCell("Alice") != "Alice" {
		t.Fatal("ordinary cell changed")
	}
}

func TestAuditQueriesAndJobBoundaries(t *testing.T) {
	s, _ := openTest(t)
	for _, actor := range []string{"admin", "operator"} {
		if err := s.Audit(actor, "test.query", "", "completed", ""); err != nil {
			t.Fatal(err)
		}
	}
	rows, total, err := s.AuditSearch(AuditFilter{Actor: "operator", Action: "test.query", Outcome: "completed", Page: 1, Size: 1}, false)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatal(rows, total, err)
	}
	if _, count, e := s.AuditSearch(AuditFilter{To: "0001-01-01T00:00:00Z", Page: 1, Size: 20}, false); e != nil || count != 0 {
		t.Fatal("explicit zero date treated as absent", count, e)
	}
	at := rows[0].At.Format(time.RFC3339Nano)
	if _, count, e := s.AuditSearch(AuditFilter{From: at, To: at, Page: 1, Size: 20}, false); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	for _, filter := range []AuditFilter{{Page: 0, Size: 20}, {Page: 1, Size: 101}, {Page: 1, Size: 20, From: "not-a-date"}, {Page: 1, Size: 20, From: "2027-01-01T00:00:00Z", To: "2026-01-01T00:00:00Z"}} {
		if _, _, e := s.AuditSearch(filter, false); e == nil {
			t.Fatal(filter)
		}
	}
	s.mu.Lock()
	for len(s.audit) < 10001 {
		e := NewEvent("admin", "test.bulk", "", "completed", "")
		s.sequence++
		e.Sequence = s.sequence
		s.audit = append(s.audit, e)
	}
	s.mu.Unlock()
	if _, _, err = s.AuditSearch(AuditFilter{Page: 1, Size: 20}, true); !errors.Is(err, ErrLimit) {
		t.Fatal("export silently truncated", err)
	}
	source := JobSource{Name: "sample.png", SHA256: Digest([]byte("source")), Width: 512, Height: 512}
	tooMany := make([]JobSource, 6)
	for i := range tooMany {
		tooMany[i] = source
	}
	if _, err = s.NewJob("too-many-images-001", "admin", tooMany, recipients, "blind", "", nil); err == nil {
		t.Fatal("source limit")
	}
	if _, err = s.NewJob("duplicate-image-001", "admin", []JobSource{source, source}, recipients, "blind", "", nil); err == nil {
		t.Fatal("duplicate original")
	}
}

func TestRecipientTemplateIsNotRecursive(t *testing.T) {
	text, err := VisibleText("{name} / {recipient_id}", Recipient{ID: "001", Name: "User {recipient_id}"})
	if err != nil || text != "User {recipient_id} / 001" {
		t.Fatal(text, err)
	}
}

func TestDegradedAuditBlocksNewRegenerationButFinishesInflight(t *testing.T) {
	s, _ := openTest(t)
	digest := strings.Repeat("a", 64)
	first, _, err := s.Begin("quota-first-request", digest, "admin", recipients)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(first.ID, ""); err != nil {
		t.Fatal(err)
	}
	inflight, _, err := s.Begin("quota-second-request", digest, "admin", recipients)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.auditFault = true
	s.mu.Unlock()
	if _, _, err = s.Resume(first.ID, digest); !errors.Is(err, ErrLimit) {
		t.Fatal("regeneration bypassed degraded audit", err)
	}
	if _, _, err = s.Begin("quota-first-request", digest, "admin", recipients); !errors.Is(err, ErrLimit) {
		t.Fatal("idempotent regeneration bypassed degraded audit", err)
	}
	if err = s.Finish(inflight.ID, ""); err != nil {
		t.Fatal("inflight finalization was blocked", err)
	}
	records, _, err := s.Batches(inflight.ID, 1, 20)
	if err != nil || len(records) != 1 || !records[0].Generated {
		t.Fatal(records, err)
	}
}
