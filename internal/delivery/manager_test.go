package delivery

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
)

var master = bytes.Repeat([]byte{42}, 32)
var legacy = bytes.Repeat([]byte{7}, 32)

func setup(t *testing.T) (*distribution.Store, *Manager, string, chan struct{}) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "data")
	s, err := distribution.Open(dir, master, legacy)
	if err != nil {
		t.Fatal(err)
	}
	slots := make(chan struct{}, 2)
	m, err := New(s, slots)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close(); s.Close() })
	return s, m, dir, slots
}
func images(t *testing.T) ([]string, []distribution.JobSource) {
	t.Helper()
	img, err := blind.Load("../blind/testdata/clean.png")
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	sources := []distribution.JobSource{}
	for i := 0; i < 2; i++ {
		copy := image.NewNRGBA(img.Bounds())
		draw.Draw(copy, copy.Bounds(), img, img.Bounds().Min, draw.Src)
		copy.Set(1, 1, color.NRGBA{uint8(30 + i*80), 150, 60, 255})
		path := filepath.Join(t.TempDir(), "source.png")
		if err = blind.SaveNew(path, copy); err != nil {
			t.Fatal(err)
		}
		hash, _, err := fileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		sources = append(sources, distribution.JobSource{Name: "same-name.png", SHA256: hash, Width: img.Bounds().Dx(), Height: img.Bounds().Dy()})
	}
	return paths, sources
}
func waitJob(t *testing.T, m *Manager, id string, status string) distribution.Job {
	t.Helper()
	until := time.Now().Add(40 * time.Second)
	for time.Now().Before(until) {
		j, err := m.View(id)
		if err != nil {
			t.Fatal(err)
		}
		if j.Status == status {
			return j
		}
		if j.Status == "failed" && status != "failed" {
			t.Fatal(j.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
	j, _ := m.View(id)
	t.Fatalf("expected %s, got %s/%s", status, j.Status, j.Error)
	return j
}
func TestMatrixArchiveRestartAndExpiry(t *testing.T) {
	s, m, dir, slots := setup(t)
	paths, sources := images(t)
	originals := [][]byte{}
	for _, path := range paths {
		b, _ := os.ReadFile(path)
		originals = append(originals, b)
	}
	recipients := []distribution.Recipient{{ID: "1", Name: "Alice"}, {ID: "2", Name: "名字"}, {ID: "3", Name: "=FORMULA"}}
	j, err := s.NewJob("real-matrix-request", "admin", sources, recipients, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, src := range j.Sources {
		if _, err = m.Upload(j.ID, src.ID, "", paths[i]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = m.Start(j.ID, "admin", false); err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, m, j.ID, "completed")
	if !done.Generated || done.Done != 6 || done.DownloadState != "ready" {
		t.Fatal(done)
	}
	f, _, err := m.OpenArchive(j.ID)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(f, done.ArchiveBytes)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	var sample []byte
	for _, entry := range archive.File {
		if filepath.Ext(entry.Name) != ".png" {
			continue
		}
		r, _ := entry.Open()
		img, err := png.Decode(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		result, keyID, err := s.Verify(img, "")
		if err != nil {
			t.Fatal(err)
		}
		record, err := s.Lookup(result, keyID)
		if err != nil || record == nil || record.JobID != j.ID || record.SourceSHA256 == "" {
			t.Fatal(record, err)
		}
		if sample == nil {
			var b bytes.Buffer
			png.Encode(&b, img)
			sample = b.Bytes()
		}
		count++
	}
	f.Close()
	if count != 6 {
		t.Fatal(count)
	}
	for i, path := range paths {
		after, _ := os.ReadFile(path)
		if !bytes.Equal(after, originals[i]) {
			t.Fatal("original changed")
		}
	}
	if _, err = os.Stat(m.inputDir(j.ID)); !os.IsNotExist(err) {
		t.Fatal("inputs retained", err)
	}
	m.Close()
	s.Close()
	reopened, err := distribution.Open(dir, master, legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	next, err := New(reopened, slots)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	f, again, err := next.OpenArchive(j.ID)
	if err != nil || again.ArchiveSHA256 != done.ArchiveSHA256 {
		t.Fatal(again, err)
	}
	actual, _, err := fileHash(next.archivePath(j.ID))
	f.Close()
	if err != nil || actual != done.ArchiveSHA256 {
		t.Fatal(actual, err)
	}
	next.Close()
	next.now = func() time.Time { return done.ExpiresAt.Add(time.Second) }
	if _, _, err = next.OpenArchive(j.ID); !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}
	if err = next.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(next.archivePath(j.ID)); !os.IsNotExist(err) {
		t.Fatal("expired archive retained")
	}
	img, _ := png.Decode(bytes.NewReader(sample))
	result, keyID, err := reopened.Verify(img, "")
	if err != nil {
		t.Fatal(err)
	}
	record, err := reopened.Lookup(result, keyID)
	if err != nil || record == nil || !record.Generated {
		t.Fatal("expiry revoked registration", record, err)
	}
}
func TestCancelRetryAndTamperedArchive(t *testing.T) {
	s, m, _, slots := setup(t)
	paths, sources := images(t)
	recipients := []distribution.Recipient{}
	for i := 0; i < 6; i++ {
		recipients = append(recipients, distribution.Recipient{ID: string(rune('a' + i)), Name: "User"})
	}
	j, err := s.NewJob("cancel-retry-request", "admin", sources[:1], recipients, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0]); err != nil {
		t.Fatal(err)
	}
	slots <- struct{}{}
	slots <- struct{}{}
	if _, err = m.Start(j.ID, "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Cancel(j.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	<-slots
	<-slots
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0], j.UploadToken); !errors.Is(err, distribution.ErrConflict) {
		t.Fatal("late cancelled upload", err)
	}

	if _, _, err = m.OpenArchive(j.ID); !errors.Is(err, distribution.ErrJobState) {
		t.Fatal("partial served", err)
	}
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(j.ID, "admin", true); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, j.ID, "processing")
	if _, err = m.Cancel(j.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	cancelled := waitJob(t, m, j.ID, "cancelled")
	if cancelled.Generated {
		t.Fatal(cancelled)
	}
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(j.ID, "admin", true); err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, m, j.ID, "completed")
	if done.Copies[0].ID != j.Copies[0].ID || done.CreatedAt != j.CreatedAt || done.Attempts != 3 {
		t.Fatal("retry changed identity", done)
	}
	data, _ := os.ReadFile(m.archivePath(j.ID))
	data[len(data)/2] ^= 1
	os.WriteFile(m.archivePath(j.ID), data, 0600)
	if _, _, err = m.OpenArchive(j.ID); !errors.Is(err, ErrArchive) {
		t.Fatal(err)
	}
	view, _ := m.View(j.ID)
	if view.DownloadState != "unavailable" || !view.Generated {
		t.Fatal(view)
	}
}
func TestDiskBudgetAndUploadIntegrity(t *testing.T) {
	s, m, dir, _ := setup(t)
	paths, sources := images(t)
	j, err := s.NewJob("upload-budget-request", "admin", sources[:1], []distribution.Recipient{{ID: "1", Name: "A"}}, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[1]); !errors.Is(err, distribution.ErrConflict) {
		t.Fatal(err)
	}
	sparse := filepath.Join(dir, "jobs-inputs", "budget")
	f, err := os.OpenFile(sparse, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(distribution.MaxTaskDiskBytes)
	f.Close()
	defer os.Remove(sparse)
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0]); !errors.Is(err, distribution.ErrLimit) {
		t.Fatal(err)
	}
}

func TestArchiveLimitFailsWholeTask(t *testing.T) {
	s, m, _, _ := setup(t)
	m.archiveLimit = 1024 // Restricted test budget exercises the real PNG/ZIP failure path.
	paths, sources := images(t)
	j, err := s.NewJob("archive-limit-request", "admin", sources[:1], []distribution.Recipient{{ID: "1", Name: "A"}}, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(j.ID, "admin", false); err != nil {
		t.Fatal(err)
	}
	failed := waitJob(t, m, j.ID, "failed")
	if failed.Generated || failed.Error != "platform_limit" {
		t.Fatal(failed)
	}
	if _, _, err = m.OpenArchive(j.ID); !errors.Is(err, distribution.ErrJobState) {
		t.Fatal(err)
	}
	if _, err = os.Stat(m.archivePath(j.ID)); !os.IsNotExist(err) {
		t.Fatal("partial archive served")
	}
}

func TestRetryLogicalSourceBudgetAfterCleanup(t *testing.T) {
	s, m, _, _ := setup(t)
	paths, _ := images(t)
	encoded, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	size := distribution.MaxSourceBytes / 3
	files := []string{}
	sources := []distribution.JobSource{}
	for i := 0; i < 3; i++ {
		path := filepath.Join(t.TempDir(), "padded.png")
		f, e := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Write(encoded); e != nil {
			t.Fatal(e)
		}
		if e = f.Truncate(size); e != nil {
			t.Fatal(e)
		}
		if _, e = f.WriteAt([]byte{byte(i + 1)}, size-1); e != nil {
			t.Fatal(e)
		}
		f.Close()
		hash, _, e := fileHash(path)
		if e != nil {
			t.Fatal(e)
		}
		files = append(files, path)
		sources = append(sources, distribution.JobSource{Name: "padded.png", SHA256: hash, Width: 512, Height: 512})
	}
	j, err := s.NewJob("reupload-source-budget", "admin", sources, []distribution.Recipient{{ID: "1", Name: "A"}}, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, src := range j.Sources {
		if _, err = m.Upload(j.ID, src.ID, "", files[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.RecoverJobs(); err != nil {
		t.Fatal(err)
	}
	if err = m.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", files[0]); err != nil {
		t.Fatal("same source counted twice after cleanup", err)
	}
}
