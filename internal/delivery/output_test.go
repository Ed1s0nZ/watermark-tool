package delivery

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"image/jpeg"
	"io"
	"strings"
	"testing"
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
