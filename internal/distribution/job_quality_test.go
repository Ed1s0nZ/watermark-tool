package distribution

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"watermark-tool/internal/quality"
)

func TestQualityPolicyFingerprintAndOldJobCompatibility(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	s, err := Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	sources := []JobSource{{Name: "raw.png", SHA256: strings.Repeat("a", 64), Width: 512, Height: 512}}
	recipients := []Recipient{{ID: "1", Name: "Alice"}}
	old, err := s.NewJob("old-no-policy-request", "admin", sources, recipients, "blind", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(struct {
		Actor      string
		Sources    []JobSource
		Recipients []Recipient
		Mode, Text string
		Style      map[string]any
	}{"admin", sources, recipients, "blind", "", nil})
	if s.jobs[0].Fingerprint != Digest(raw) {
		t.Fatal("historical request fingerprint changed")
	}
	if old.QualityPolicy != nil {
		t.Fatal("fabricated old policy")
	}
	if _, err = s.JobInput(old.ID, old.Sources[0].ID, "", sources[0].SHA256, 100); err != nil {
		t.Fatal(err)
	}
	if _, err = s.QueueJob(old.ID, "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.StartJob(old.ID); err != nil {
		t.Fatal(err)
	}
	s.JobProgress(old.ID, 1, "packing")
	if _, err = s.FinishJob(old.ID, "completed", "", strings.Repeat("b", 64), 100); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	loaded, e := s.Job(old.ID)
	if e != nil || !loaded.Generated || loaded.Copies[0].Quality != nil {
		t.Fatal(e, loaded)
	}
	repeated, e := s.NewJob("old-no-policy-request", "admin", sources, recipients, "blind", "", nil)
	if e != nil || repeated.ID != old.ID {
		t.Fatal("old idempotency", e)
	}
	defaultRepeated, e := s.NewJob("old-no-policy-request", "admin", sources, recipients, "blind", "", nil, quality.Policy{})
	if e != nil || defaultRepeated.ID != old.ID {
		t.Fatal("equivalent display-only policy conflicts with historical default", e)
	}
	gate := 35.0
	j, err := s.NewJob("new-policy-request-1", "admin", sources, recipients, "blind", "", nil, quality.Policy{MinimumPSNR: &gate})
	if err != nil {
		t.Fatal(err)
	}
	gate = 36
	if _, err = s.NewJob("new-policy-request-1", "admin", sources, recipients, "blind", "", nil, quality.Policy{MinimumPSNR: &gate}); !errors.Is(err, ErrConflict) {
		t.Fatal("policy omitted from fingerprint", err)
	}
	j.QualityPolicy.MinimumPSNR = &gate
	next, _ := s.Job(j.ID)
	if *next.QualityPolicy.MinimumPSNR != 35 {
		t.Fatal("caller mutation")
	}
}
