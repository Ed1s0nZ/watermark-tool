package distribution

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"watermark-tool/internal/composition"
	"watermark-tool/internal/output"
)

func TestArtworkOutputSnapshotsAndLegacyDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s, e := Open(dir, testMaster, testLegacy)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { s.Close() }()
	sources := []JobSource{{Name: "raw.png", SHA256: strings.Repeat("a", 64), Width: 512, Height: 512}}
	r := []Recipient{{ID: "1", Name: "A"}}
	style := map[string]any{"mode": "single", "position": "bottomRight", "size": 28, "opacity": 0.35, "color": "#365bdb", "angle": -30}
	old, e := s.NewJob("artwork-old-request", "admin", sources, r, "dual", "{name}", style)
	if e != nil {
		t.Fatal(e)
	}
	same, e := s.NewJobWithOptions("artwork-old-request", "admin", sources, r, "dual", "{name}", style, JobOptions{Output: &output.Options{Format: "png"}, Artwork: &composition.Artwork{Layers: "text"}})
	if e != nil || same.ID != old.ID {
		t.Fatal("default fingerprint", e)
	}
	a := &composition.Artwork{Layers: "logo", Logo: &composition.Logo{SHA256: strings.Repeat("b", 64), Position: "center", WidthRatio: 0.15, Opacity: 0.35}, Exclusions: map[string][]composition.Rect{sources[0].SHA256: {{X: 0, Y: 0, Width: 0.2, Height: 0.2}}}}
	options := JobOptions{Artwork: a, Output: &output.Options{Format: "jpeg", JPEGQuality: 90}}
	j, e := s.NewJobWithOptions("artwork-new-request", "admin", sources, r, "dual", "", style, options)
	if e != nil {
		t.Fatal(e)
	}
	a.Logo.Opacity = 0.5
	if _, e = s.NewJobWithOptions("artwork-new-request", "admin", sources, r, "dual", "", style, options); !errors.Is(e, ErrConflict) {
		t.Fatal("changed logo layout", e)
	}
	a.Logo.Opacity = 0.35
	a.Logo.SHA256 = strings.Repeat("c", 64)
	if _, e = s.NewJobWithOptions("artwork-new-request", "admin", sources, r, "dual", "", style, options); !errors.Is(e, ErrConflict) {
		t.Fatal("changed logo identity", e)
	}
	a.Logo.SHA256 = j.Artwork.Logo.SHA256
	options.Output.JPEGQuality = 100
	if _, e = s.NewJobWithOptions("artwork-new-request", "admin", sources, r, "dual", "", style, options); !errors.Is(e, ErrConflict) {
		t.Fatal("changed output", e)
	}
	s.Close()
	s, e = Open(dir, testMaster, testLegacy)
	if e != nil {
		t.Fatal(e)
	}
	loaded, e := s.Job(j.ID)
	if e != nil || loaded.Artwork.Logo.Opacity != 0.35 || loaded.Output.JPEGQuality != 90 {
		t.Fatal("pinned persisted", e)
	}
}
