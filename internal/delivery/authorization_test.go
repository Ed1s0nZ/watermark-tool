package delivery

import (
	"bytes"
	"errors"
	"image/jpeg"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"watermark-tool/internal/distribution"
)

func TestAuthorizationAtStartDispatchAndDownload(t *testing.T) {
	s, m, _, slots := setup(t)
	paths, sources := images(t)
	var allowed atomic.Bool
	m.authorize = func(distribution.Job) error {
		if !allowed.Load() {
			return errors.New("revoked")
		}
		return nil
	}
	j, err := s.NewJob(strings.Repeat("first", 5), "operator", sources[:1], []distribution.Recipient{{ID: "1", Name: "Alice"}}, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	j, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(j.ID, "operator", false); !errors.Is(err, ErrGate) {
		t.Fatal("unapproved start", err)
	}
	// Hold the global processing slots so revocation happens after queueing but before dispatch.
	slots <- struct{}{}
	slots <- struct{}{}
	allowed.Store(true)
	if _, err = m.Start(j.ID, "operator", false); err != nil {
		t.Fatal(err)
	}
	allowed.Store(false)
	<-slots
	<-slots
	failed := waitJob(t, m, j.ID, "failed")
	if failed.Error != "approval_required" || failed.Generated {
		t.Fatal(failed)
	}
	// A separate task can execute, but a subsequent revocation prevents downloading its ZIP.
	j, err = s.NewJob(strings.Repeat("second", 4), "operator", sources[:1], []distribution.Recipient{{ID: "1", Name: "Alice"}}, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0]); err != nil {
		t.Fatal(err)
	}
	allowed.Store(true)
	if _, err = m.Start(j.ID, "operator", false); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, j.ID, "completed")
	allowed.Store(false)
	if f, _, err := m.OpenArchive(j.ID); !errors.Is(err, ErrGate) {
		if f != nil {
			f.Close()
		}
		t.Fatal("revoked download", err)
	}
}
func TestApprovalThumbnailActualInputAndTamper(t *testing.T) {
	s, m, _, _ := setup(t)
	paths, sources := images(t)
	j, err := s.NewJob(strings.Repeat("preview", 3), "operator", sources[:1], []distribution.Recipient{{ID: "1", Name: "Alice"}}, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApprovalInputs(j.ID); err == nil {
		t.Fatal("missing input accepted")
	}
	if _, err = m.Upload(j.ID, j.Sources[0].ID, "", paths[0]); err != nil {
		t.Fatal(err)
	}
	raw, err := m.ApprovalThumbnail(j.ID, j.Sources[0].ID, "")
	if err != nil {
		t.Fatal(err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width > 256 || config.Height > 256 || len(raw) > 128<<10 {
		t.Fatal(config, err)
	}
	if _, err = m.ApprovalThumbnail(j.ID, "../../other", ""); err == nil {
		t.Fatal("unknown input accepted")
	}
	if _, err = m.ApprovalThumbnail(j.ID, j.Sources[0].ID, "../../other"); err == nil {
		t.Fatal("unknown copy accepted")
	}
	if err = os.WriteFile(m.inputPath(j.ID, j.Sources[0].ID, ""), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApprovalInputs(j.ID); err == nil {
		t.Fatal("tampered input accepted")
	}
}
