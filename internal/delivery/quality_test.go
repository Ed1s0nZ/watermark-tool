package delivery

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/quality"
)

func TestQualityMatrixAndGate(t *testing.T) {
	s, m, dir, slots := setup(t)
	paths, sources := images(t)
	recipients := []distribution.Recipient{{ID: "1", Name: "A"}, {ID: "2", Name: "名字"}, {ID: "3", Name: "C"}}
	// The dual inputs are independent lossless originals here; expected visible metric is infinite.
	style := map[string]any{"mode": "single", "position": "center", "size": float64(28), "opacity": 0.35, "color": "#365bdb", "angle": float64(-30)}
	j, err := s.NewJob("quality-real-matrix-1", "admin", sources, recipients, "dual", "{name}", style, quality.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	upload := func(j distribution.Job) {
		t.Helper()
		for i, src := range j.Sources {
			if _, e := m.Upload(j.ID, src.ID, "", paths[i], j.UploadToken); e != nil {
				t.Fatal(e)
			}
		}
		for _, c := range j.Copies {
			index := 0
			if c.SourceID == j.Sources[1].ID {
				index = 1
			}
			if _, e := m.Upload(j.ID, c.SourceID, c.ID, paths[index], j.UploadToken); e != nil {
				t.Fatal(e)
			}
		}
	}
	upload(j)
	if _, err = m.Start(j.ID, "admin", false); err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, m, j.ID, "completed")
	for _, c := range done.Copies {
		if c.Quality == nil || !c.Quality.Accepted || c.Quality.Visible == nil || c.Quality.Visible.Status != "infinite" || c.Quality.Embedding.Status != "finite" {
			t.Fatal(c)
		}
	}
	f, _, err := m.OpenArchive(j.ID)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(f, done.ArchiveBytes)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range z.File {
		if entry.Name == "manifest.csv" {
			r, _ := entry.Open()
			data, _ := io.ReadAll(r)
			r.Close()
			rows, e := csv.NewReader(bytes.NewReader(data)).ReadAll()
			if e != nil || len(rows) != 7 || rows[0][9] != "quality_json" {
				t.Fatal(e, rows)
			}
			found = true
		}
	}
	f.Close()
	if !found {
		t.Fatal("manifest")
	}
	m.Close()
	s.Close()
	s, err = distribution.Open(dir, master, legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	m, err = New(s, slots)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	persisted, e := s.Job(j.ID)
	if e != nil || persisted.Copies[0].Quality == nil {
		t.Fatal(e)
	}
	gate := 60.0
	j, err = s.NewJob("quality-real-matrix-2", "admin", sources, recipients, "dual", "{name}", style, quality.Policy{MinimumPSNR: &gate})
	if err != nil {
		t.Fatal(err)
	}
	gate = 20 // Caller mutation cannot change the pinned policy.
	upload(j)
	if _, err = m.Start(j.ID, "admin", false); err != nil {
		t.Fatal(err)
	}
	failed := waitJob(t, m, j.ID, "failed")
	if failed.Error != "quality_failed" || failed.Generated || failed.Copies[0].Quality == nil || failed.Copies[0].Quality.Accepted || *failed.QualityPolicy.MinimumPSNR != 60 {
		t.Fatal(failed)
	}
	if _, _, err = m.OpenArchive(j.ID); err == nil {
		t.Fatal("partial download")
	}
	if _, err = os.Stat(filepath.Join(dir, "artifacts", j.ID+".part")); !os.IsNotExist(err) {
		t.Fatal("partial file", err)
	}
	upload(failed)
	if err = m.Cleanup(); err != nil {
		t.Fatal(err)
	} // Fresh terminal retry inputs must survive the periodic cleaner.
	if _, err = m.Start(j.ID, "admin", true); err != nil {
		t.Fatal(err)
	}
	again := waitJob(t, m, j.ID, "failed")
	if again.Copies[0].ID != j.Copies[0].ID || again.CreatedAt != j.CreatedAt || again.Attempts != 2 || again.Error != "quality_failed" {
		t.Fatal("retry snapshot")
	}
	// Expire abandoned retry staging without touching the fixed failed result.
	upload(again)
	m.Close()
	m.now = func() time.Time { return time.Now().Add(16 * time.Minute) }
	if err = m.Cleanup(); err != nil {
		t.Fatal(err)
	}
	expired, e := s.Job(j.ID)
	if e != nil || distribution.HasJobInputs(expired) || expired.UploadToken == again.UploadToken || expired.Copies[0].Quality == nil {
		t.Fatal("expired staging", e)
	}
	m, err = New(s, slots)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	upload(expired)
	staged, _ := s.Job(j.ID)
	m.Close()
	m, err = New(s, slots)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	recovered, _ := s.Job(j.ID)
	if distribution.HasJobInputs(recovered) || recovered.UploadToken == staged.UploadToken || recovered.Copies[0].ID != j.Copies[0].ID {
		t.Fatal("startup staging recovery")
	}
	if _, e = os.Stat(m.inputDir(j.ID)); !os.IsNotExist(e) {
		t.Fatal("staged files survived restart", e)
	}
	old, _ := s.Job(done.ID)
	if !old.Generated {
		t.Fatal("other task registration lost")
	}
}
