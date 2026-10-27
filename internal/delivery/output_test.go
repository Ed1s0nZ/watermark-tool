package delivery

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io"
	"strings"
	"testing"
	"watermark-tool/internal/composition"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/output"
	"watermark-tool/internal/quality"
)

func TestJPEGMatrixActualArchiveAndFailedGate(t *testing.T) {
	s, m, _, _ := setup(t)
	paths, sources := images(t)
	r := []distribution.Recipient{{ID: "1", Name: "A"}, {ID: "2", Name: "名字"}, {ID: "3", Name: "C"}}
	j, e := s.NewJobWithOptions("jpeg-matrix-request", "admin", sources, r, "blind", "", nil, distribution.JobOptions{Output: &output.Options{Format: "jpeg", JPEGQuality: 90}}, quality.Policy{})
	if e != nil {
		t.Fatal(e)
	}
	for i, src := range j.Sources {
		if _, e = m.Upload(j.ID, src.ID, "", paths[i], j.UploadToken); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = m.Start(j.ID, "admin", false); e != nil {
		t.Fatal(e)
	}
	done := waitJob(t, m, j.ID, "completed")
	f, _, e := m.OpenArchive(j.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := zip.NewReader(f, done.ArchiveBytes)
	if e != nil {
		t.Fatal(e)
	}
	seen := 0
	for _, entry := range z.File {
		rd, _ := entry.Open()
		data, _ := io.ReadAll(rd)
		rd.Close()
		if entry.Name == "manifest.csv" {
			rows, e := csv.NewReader(bytes.NewReader(data)).ReadAll()
			if e != nil || len(rows) != 7 {
				t.Fatal(e)
			}
			for _, row := range rows[1:] {
				var q quality.Summary
				if json.Unmarshal([]byte(row[9]), &q) != nil || q.Format != "jpeg" || q.OutputSHA256 != distribution.Digest(findJPEG(t, z, row[0])) {
					t.Fatal(row)
				}
			}
			continue
		}
		if !strings.HasSuffix(entry.Name, ".jpg") || !bytes.HasPrefix(data, []byte{255, 216}) {
			t.Fatal(entry.Name)
		}
		img, e := jpeg.Decode(bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		result, key, e := s.Verify(img, "")
		if e != nil {
			t.Fatal(e)
		}
		record, e := s.Lookup(result, key)
		if e != nil || record == nil || record.JobID != j.ID {
			t.Fatal(e)
		}
		seen++
	}
	if seen != 6 {
		t.Fatal(seen)
	}
	gate := 60.0
	failed, e := s.NewJobWithOptions("jpeg-matrix-failed", "admin", sources, r, "blind", "", nil, distribution.JobOptions{Output: &output.Options{Format: "jpeg", JPEGQuality: 60}}, quality.Policy{MinimumPSNR: &gate})
	if e != nil {
		t.Fatal(e)
	}
	for i, src := range failed.Sources {
		if _, e = m.Upload(failed.ID, src.ID, "", paths[i], failed.UploadToken); e != nil {
			t.Fatal(e)
		}
	}
	m.Start(failed.ID, "admin", false)
	failed = waitJob(t, m, failed.ID, "failed")
	if failed.Generated || failed.Error != "quality_failed" || failed.Copies[0].Quality == nil || failed.Copies[0].Quality.Final == nil {
		t.Fatal(failed)
	}
	if _, _, e = m.OpenArchive(failed.ID); e == nil {
		t.Fatal("partial JPEG archive")
	}

}
func findJPEG(t *testing.T, z *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range z.File {
		if f.Name == name {
			r, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			b, e := io.ReadAll(r)
			if e != nil {
				t.Fatal(e)
			}
			return b
		}
	}
	t.Fatal(name)
	return nil
}

func TestJPEGArtworkInterruptedRecovery(t *testing.T) {
	s, m, dir, slots := setup(t)
	paths, sources := images(t)
	sources = sources[:1]
	art := &composition.Artwork{Layers: "logo", Logo: &composition.Logo{SHA256: strings.Repeat("b", 64), Position: "topRight", WidthRatio: 0.15, Opacity: 0.35}, Exclusions: map[string][]composition.Rect{sources[0].SHA256: {{X: 0.7, Y: 0.7, Width: 0.2, Height: 0.2}}}}
	options := distribution.JobOptions{Output: &output.Options{Format: "jpeg", JPEGQuality: 90}, Artwork: art}
	style := map[string]any{"mode": "single", "position": "bottomRight", "size": 28, "opacity": 0.35, "color": "#365bdb", "angle": -30}
	j, e := s.NewJobWithOptions("jpeg-artwork-recover", "admin", sources, []distribution.Recipient{{ID: "1", Name: "A"}}, "dual", "", style, options, quality.Policy{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Upload(j.ID, j.Sources[0].ID, "", paths[0], j.UploadToken); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Upload(j.ID, j.Sources[0].ID, j.Copies[0].ID, paths[0], j.UploadToken); e != nil {
		t.Fatal(e)
	}
	slots <- struct{}{}
	slots <- struct{}{}
	if _, e = m.Start(j.ID, "admin", false); e != nil {
		t.Fatal(e)
	}
	m.Close()
	<-slots
	<-slots
	s.Close()
	s, e = distribution.Open(dir, master, legacy)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e = New(s, slots)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	recovered, e := m.View(j.ID)
	if e != nil || recovered.Status != "interrupted" || recovered.UploadToken == j.UploadToken || recovered.Output.JPEGQuality != 90 || recovered.Artwork.Logo.SHA256 != art.Logo.SHA256 || recovered.Copies[0].ID != j.Copies[0].ID {
		t.Fatal(recovered, e)
	}
	art.Logo.SHA256 = strings.Repeat("c", 64)
	if _, e = s.NewJobWithOptions("jpeg-artwork-recover", "admin", sources, []distribution.Recipient{{ID: "1", Name: "A"}}, "dual", "", style, options, quality.Policy{}); !errors.Is(e, distribution.ErrConflict) {
		t.Fatal("wrong logo snapshot", e)
	}
	art.Logo.SHA256 = recovered.Artwork.Logo.SHA256
	art.Exclusions[sources[0].SHA256][0].X = 0.6
	if _, e = s.NewJobWithOptions("jpeg-artwork-recover", "admin", sources, []distribution.Recipient{{ID: "1", Name: "A"}}, "dual", "", style, options, quality.Policy{}); !errors.Is(e, distribution.ErrConflict) {
		t.Fatal("wrong region snapshot", e)
	}
	if _, e = m.Upload(j.ID, recovered.Sources[0].ID, "", paths[1], recovered.UploadToken); !errors.Is(e, distribution.ErrConflict) {
		t.Fatal("wrong raw", e)
	}
	if _, e = m.Upload(j.ID, recovered.Sources[0].ID, recovered.Copies[0].ID, paths[1], recovered.UploadToken); !errors.Is(e, distribution.ErrConflict) {
		t.Fatal("wrong rendered", e)
	}
	if _, e = m.Upload(j.ID, recovered.Sources[0].ID, "", paths[0], recovered.UploadToken); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Upload(j.ID, recovered.Sources[0].ID, recovered.Copies[0].ID, paths[0], recovered.UploadToken); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Start(j.ID, "admin", true); e != nil {
		t.Fatal(e)
	}
	done := waitJob(t, m, j.ID, "completed")
	if done.Copies[0].ID != j.Copies[0].ID || done.CreatedAt != j.CreatedAt || done.Output.JPEGQuality != 90 || done.Copies[0].Quality.Format != "jpeg" {
		t.Fatal(done)
	}
}
