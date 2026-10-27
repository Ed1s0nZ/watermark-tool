package filetasks

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/fileprocess"
)

func testOptions() Options {
	return Options{Team: "team-test", MetadataKey: bytes.Repeat([]byte{0x43}, 32), Slots: make(chan struct{}, 2), CurrentKey: func() (string, []byte, error) { return "key-1", bytes.Repeat([]byte{0x72}, 32), nil }, ResolveKey: func(id string) ([]byte, error) {
		if id != "key-1" {
			return nil, ErrDenied
		}
		return bytes.Repeat([]byte{0x72}, 32), nil
	}}
}
func newManager(t *testing.T, options Options) *Manager {
	t.Helper()
	m, err := New(filepath.Join(t.TempDir(), "tasks"), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}
func ingester(t *testing.T, names map[string]string) Ingester {
	t.Helper()
	sources := []filebundle.Source{}
	for name, fixture := range names {
		raw, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		original := filepath.Join(t.TempDir(), "original")
		if err = os.WriteFile(original, raw, 0600); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, filebundle.Source{RelativePath: name, PhysicalPath: original})
	}
	return func(ctx context.Context, destination string) (filebundle.Bundle, error) {
		return filebundle.Ingest(ctx, sources, destination, filebundle.DefaultLimits())
	}
}
func request() Request { return Request{Options: fileprocess.Options{Text: "team-private 标识"}} }
func waitJob(t *testing.T, m *Manager, id string, condition func(Job) bool) Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		j, err := m.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if condition(j) {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	j, _ := m.Get(id)
	t.Fatalf("timeout: %+v", j)
	return Job{}
}
func settled(t *testing.T, m *Manager, id string) Job {
	return waitJob(t, m, id, func(j Job) bool {
		m.mu.Lock()
		active := m.running[id] != nil
		m.mu.Unlock()
		return terminal(j.Status) && j.ArchiveAvailable && !active
	})
}
func TestActualMixedFolderPartialArchiveAndFailedOnlyRetry(t *testing.T) {
	m := newManager(t, testOptions())
	j, err := m.Create(t.Context(), "alice", request(), ingester(t, map[string]string{"资料/document.docx": "../fileprocess/testdata/source.docx", "资料/slides.pptx": "../fileprocess/testdata/source.pptx", "资料/book.xlsx": "../fileprocess/testdata/source.xlsx", "open/document.odt": "../fileprocess/testdata/source.odt", "legacy.rtf": "../fileprocess/testdata/source.rtf", "unsupported.exe": "../fileprocess/testdata/source.rtf", "pages/source.pdf": "../pdfmark/testdata/source.pdf"}))
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != "ready" || j.Revision != 1 || len(j.Items) != 7 {
		t.Fatal(j)
	}
	before, err := Snapshot(j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.ValidateInputs(t.Context(), j.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(j.ID, j.Revision); err != nil {
		t.Fatal(err)
	}
	j = settled(t, m, j.ID)
	if j.Status != "partial" {
		t.Fatal(j)
	}
	firstAttempts := map[string]int{}
	mtimes := map[string]time.Time{}
	success := 0
	for _, item := range j.Items {
		firstAttempts[item.Source.ID] = len(item.Attempts)
		if item.Status == "completed" {
			success++
			info, e := os.Stat(filepath.Join(m.path("outputs", j.ID), item.Source.ID))
			if e != nil {
				t.Fatal(e)
			}
			mtimes[item.Source.ID] = info.ModTime()
		} else if item.Error != fileprocess.ErrUnsupported.Error() {
			t.Fatal(item)
		}
	}
	if success != 6 {
		t.Fatal(success)
	}
	after, _ := Snapshot(j)
	if before != after {
		t.Fatal("snapshot changed with results")
	}
	f, download, err := m.OpenArchive(t.Context(), j.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(f)
	f.Close()
	if err != nil || int64(len(raw)) != download.ArchiveBytes {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	files, reports := 0, 0
	for _, entry := range reader.File {
		if strings.HasPrefix(entry.Name, "files/") {
			files++
		}
		if strings.HasPrefix(entry.Name, "reports/") {
			reports++
		}
		if strings.Contains(entry.Name, "unsupported.exe") {
			t.Fatal("failed file included")
		}
		r, e := entry.Open()
		if e != nil {
			t.Fatal(e)
		}
		_, e = io.Copy(io.Discard, r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	if files != 6 || reports != 6 {
		t.Fatal(files, reports)
	}
	for attempt := 2; attempt <= MaxAttempts; attempt++ {
		if _, err = m.Start(j.ID, j.Revision); err != nil {
			t.Fatal(err)
		}
		j = settled(t, m, j.ID)
		for _, item := range j.Items {
			if item.Status == "completed" {
				info, _ := os.Stat(filepath.Join(m.path("outputs", j.ID), item.Source.ID))
				if len(item.Attempts) != firstAttempts[item.Source.ID] || !info.ModTime().Equal(mtimes[item.Source.ID]) {
					t.Fatal("successful file was reprocessed")
				}
			} else if len(item.Attempts) != attempt {
				t.Fatal(item)
			}
		}
	}
	if _, err = m.Start(j.ID, j.Revision); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(m.root, "tasks-v1.enc"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"team-private", "alice", "资料", "source.docx"} {
		if bytes.Contains(stored, []byte(secret)) {
			t.Fatal("plaintext metadata leaked", secret)
		}
	}
}
func TestRealZIPAndSafeDirectories(t *testing.T) {
	m := newManager(t, testOptions())
	var raw bytes.Buffer
	writer := zip.NewWriter(&raw)
	for _, name := range []string{"folder/empty/", "folder/source.rtf"} {
		entry, e := writer.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if strings.HasSuffix(name, ".rtf") {
			source, e := os.ReadFile("../fileprocess/testdata/source.rtf")
			if e != nil {
				t.Fatal(e)
			}
			entry.Write(source)
		}
	}
	writer.Close()
	name := filepath.Join(t.TempDir(), "original.zip")
	os.WriteFile(name, raw.Bytes(), 0600)
	j, err := m.Create(t.Context(), "alice", request(), func(ctx context.Context, dest string) (filebundle.Bundle, error) {
		archive, e := filebundle.Open(ctx, name, filebundle.DefaultLimits())
		if e != nil {
			return filebundle.Bundle{}, e
		}
		defer archive.Close()
		return archive.Extract(ctx, dest)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(j.ID, j.Revision); err != nil {
		t.Fatal(err)
	}
	j = settled(t, m, j.ID)
	if j.Status != "completed" || len(j.Directories) != 1 {
		t.Fatal(j)
	}
	f, _, err := m.OpenArchive(t.Context(), j.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, _ := f.Stat()
	reader, err := zip.NewReader(f, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	empty := false
	for _, entry := range reader.File {
		empty = empty || entry.Name == "files/folder/empty/"
	}
	if !empty {
		t.Fatal("empty directory lost")
	}
}
func TestApprovalGateChangeSourceAndArchiveTamper(t *testing.T) {
	var denied atomic.Bool
	options := testOptions()
	options.Authorize = func(Job) error {
		if denied.Load() {
			return ErrDenied
		}
		return nil
	}
	m := newManager(t, options)
	j, err := m.Create(t.Context(), "alice", request(), ingester(t, map[string]string{"source.docx": "../fileprocess/testdata/source.docx"}))
	if err != nil {
		t.Fatal(err)
	}
	denied.Store(true)
	if _, err = m.Start(j.ID, j.Revision); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	denied.Store(false)
	if _, err = m.Start(j.ID, j.Revision); err != nil {
		t.Fatal(err)
	}
	j = settled(t, m, j.ID)
	denied.Store(true)
	if _, _, err = m.OpenArchive(t.Context(), j.ID); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	denied.Store(false)
	f, err := os.OpenFile(m.path("archives", j.ID+".zip"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	one := make([]byte, 1)
	if _, err = f.ReadAt(one, 5); err != nil {
		t.Fatal(err)
	}
	one[0] ^= 1
	f.WriteAt(one, 5)
	f.Close()
	if _, _, err = m.OpenArchive(t.Context(), j.ID); !errors.Is(err, ErrStore) {
		t.Fatal(err)
	}
	j2, err := m.Create(t.Context(), "alice", request(), ingester(t, map[string]string{"source.docx": "../fileprocess/testdata/source.docx"}))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(m.source(j2.ID, j2.Items[0].Source).PhysicalPath, []byte("changed"), 0600)
	if _, err = m.ValidateInputs(t.Context(), j2.ID); !errors.Is(err, fileprocess.ErrSource) {
		t.Fatal(err)
	}
	if _, err = m.Start(j2.ID, j2.Revision); err != nil {
		t.Fatal(err)
	}
	j2 = settled(t, m, j2.ID)
	if j2.Status != "failed" || j2.Items[0].Error != fileprocess.ErrSource.Error() {
		t.Fatal(j2)
	}
}
func TestRecoveryExpiryAndNoOverwrite(t *testing.T) {
	var clock atomic.Int64
	clock.Store(time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC).Unix())
	options := testOptions()
	options.Now = func() time.Time { return time.Unix(clock.Load(), 0) }
	root := filepath.Join(t.TempDir(), "tasks")
	m, err := New(root, options)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := New(root, options); !errors.Is(err, ErrStore) {
		if other != nil {
			other.Close()
		}
		t.Fatal("concurrent writer accepted", err)
	}
	j, err := m.Create(t.Context(), "alice", request(), ingester(t, map[string]string{"source.rtf": "../fileprocess/testdata/source.rtf"}))
	if err != nil {
		t.Fatal(err)
	}
	// Occupy both real work slots to leave an actual persisted queued job.
	options.Slots <- struct{}{}
	options.Slots <- struct{}{}
	if _, err = m.Start(j.ID, j.Revision); err != nil {
		t.Fatal(err)
	}
	m.Close()
	<-options.Slots
	<-options.Slots
	m, err = New(root, options)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	j, err = m.Get(j.ID)
	if err != nil || j.Status != "interrupted" || j.Items[0].Status != "interrupted" {
		t.Fatal(j, err)
	}
	if _, err = m.Start(j.ID, j.Revision); err != nil {
		t.Fatal(err)
	}
	j = settled(t, m, j.ID)
	if j.Status != "completed" {
		t.Fatal(j)
	}
	clock.Add(int64(InputTTL/time.Second) + 1)
	if err = m.Cleanup(); err != nil {
		t.Fatal(err)
	}
	j, _ = m.Get(j.ID)
	if j.InputsAvailable || !j.ArchiveAvailable || j.Items[0].Status != "completed" {
		t.Fatal(j)
	}
	clock.Add(3601)
	if err = m.Cleanup(); err != nil {
		t.Fatal(err)
	}
	j, _ = m.Get(j.ID)
	if j.ArchiveAvailable {
		t.Fatal(j)
	}
	if _, _, err = m.OpenArchive(t.Context(), j.ID); !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}
	if err = m.Delete(j.ID, j.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Get(j.ID); !errors.Is(err, ErrMissing) {
		t.Fatal(err)
	}
}
func TestWrongKeyTamperAndFailedMetadataWriteFailClosed(t *testing.T) {
	options := testOptions()
	root := filepath.Join(t.TempDir(), "tasks")
	m, err := New(root, options)
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
	bad := options
	bad.MetadataKey = bytes.Repeat([]byte{0x21}, 32)
	if opened, e := New(root, bad); !errors.Is(e, ErrStore) {
		if opened != nil {
			opened.Close()
		}
		t.Fatal(e)
	}
	bad = options
	bad.Team = "other-team"
	if opened, e := New(root, bad); !errors.Is(e, ErrStore) {
		if opened != nil {
			opened.Close()
		}
		t.Fatal(e)
	}
	m, err = New(root, options)
	if err != nil {
		t.Fatal(err)
	}
	m.write = func(string, []byte) error { return os.ErrPermission }
	if _, err = m.Create(t.Context(), "alice", request(), ingester(t, map[string]string{"source.rtf": "../fileprocess/testdata/source.rtf"})); !errors.Is(err, ErrStore) {
		t.Fatal(err)
	}
	if _, err = m.List(); !errors.Is(err, ErrStore) {
		t.Fatal(err)
	}
	m.Close()
	path := filepath.Join(root, "tasks-v1.enc")
	raw, _ := os.ReadFile(path)
	raw[len(raw)-1] ^= 1
	os.WriteFile(path, raw, 0600)
	if opened, e := New(root, options); !errors.Is(e, ErrStore) {
		if opened != nil {
			opened.Close()
		}
		t.Fatal(e)
	}
}
