package filetasks

import (
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCancelRunningWaitsForWriterAndKeepsSource(t *testing.T) {
	m := newManager(t, testOptions())
	img := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{100, 120, 140, 255}), image.Point{}, draw.Src)
	original := filepath.Join(t.TempDir(), "original.png")
	f, err := os.Create(original)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	j, err := m.Create(t.Context(), "alice", request(), ingester(t, map[string]string{"image.png": original}))
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan Job, 1)
	release := make(chan struct{})
	notified := false
	m.mu.Lock()
	m.write = func(path string, raw []byte) error {
		if err := atomicWrite(path, raw); err != nil {
			return err
		}
		plain, e := m.aead.Open(nil, raw[:m.aead.NonceSize()], raw[m.aead.NonceSize():], m.aad)
		if e != nil {
			return e
		}
		var next state
		if e = json.Unmarshal(plain, &next); e != nil {
			return e
		}
		current := next.Jobs[j.ID]
		if !notified && len(current.Items) > 0 && current.Items[0].Status == "running" {
			notified = true
			started <- current
			select {
			case <-release:
			case <-m.ctx.Done():
			}
		}
		return nil
	}
	m.mu.Unlock()
	if _, err = m.Start(j.ID, j.Revision); err != nil {
		t.Fatal(err)
	}
	var executing Job
	select {
	case executing = <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("execution did not start")
	}
	cancelledResult := make(chan error, 1)
	go func() { _, e := m.Cancel(j.ID, executing.Revision); cancelledResult <- e }()
	close(release)
	select {
	case err = <-cancelledResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel request blocked")
	}
	j = settled(t, m, j.ID)
	if j.Status != "cancelled" || j.Items[0].Status != "cancelled" || j.Items[0].Result != nil {
		t.Fatal(j)
	}
	if _, err = os.Stat(m.source(j.ID, j.Items[0].Source).PhysicalPath); err != nil {
		t.Fatal("source removed before writer completed", err)
	}
	if _, err = os.Stat(filepath.Join(m.path("outputs", j.ID), j.Items[0].Source.ID)); !os.IsNotExist(err) {
		t.Fatal("cancelled output remains", err)
	}
	if len(j.Items[0].Attempts) != 1 || j.Items[0].Attempts[0].Status != "cancelled" {
		t.Fatal(j.Items[0])
	}
}

func TestRevocationAfterActualProcessingPreventsCommitAndDelivery(t *testing.T) {
	options := testOptions()
	var checks atomic.Int64
	options.Authorize = func(Job) error {
		if checks.Add(1) >= 4 {
			return ErrDenied
		}
		return nil
	}
	m := newManager(t, options)
	j, err := m.Create(t.Context(), "alice", request(), ingester(t, map[string]string{"source.rtf": "../fileprocess/testdata/source.rtf"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(j.ID, j.Revision); err != nil {
		t.Fatal(err)
	}
	j = waitJob(t, m, j.ID, func(j Job) bool {
		m.mu.Lock()
		active := m.running[j.ID] != nil
		m.mu.Unlock()
		return terminal(j.Status) && !active
	})
	if j.ArchiveAvailable || j.Items[0].Status != "failed" || j.Items[0].Result != nil || j.Items[0].Error != ErrDenied.Error() {
		t.Fatal(j)
	}
	if _, _, err = m.OpenArchive(t.Context(), j.ID); err == nil {
		t.Fatal("revoked result delivered")
	}
	if _, err = os.Stat(filepath.Join(m.path("outputs", j.ID), j.Items[0].Source.ID)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestBoundsRejectBeforeCommittingInputs(t *testing.T) {
	options := testOptions()
	m := newManager(t, options)
	ingest := ingester(t, map[string]string{"source.rtf": "../fileprocess/testdata/source.rtf"})
	huge := request()
	huge.Options.Text = strings.Repeat("x", MaxJobBytes)
	if _, err := m.Create(t.Context(), "alice", huge, ingest); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	options.Slots <- struct{}{}
	options.Slots <- struct{}{}
	if _, err := m.Create(t.Context(), "alice", request(), ingest); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	<-options.Slots
	<-options.Slots
	sparse, err := os.Create(filepath.Join(m.root, "quota-test"))
	if err != nil {
		t.Fatal(err)
	}
	if err = sparse.Truncate(MaxDiskBytes); err != nil {
		t.Fatal(err)
	}
	sparse.Close()
	if _, err = m.Create(t.Context(), "alice", request(), ingest); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(m.root, "quota-test"))
	jobs, err := m.List()
	if err != nil || len(jobs) != 0 {
		t.Fatal(jobs, err)
	}
	entries, err := os.ReadDir(m.path("inputs", ""))
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}
