package delivery

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/quality"
)

type limitedWriter struct {
	out       io.Writer
	remaining int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, distribution.ErrLimit
	}
	n, err := w.out.Write(p)
	w.remaining -= int64(n)
	return n, err
}
func cell(s string) string { return distribution.CSVCell(s) }

func (m *Manager) archive(ctx context.Context, j distribution.Job, key []byte) (hash string, size int64, err error) {
	path := filepath.Join(m.root, "artifacts", j.ID+".part")
	defer func() {
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", 0, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()
	z := zip.NewWriter(&limitedWriter{f, m.archiveLimit})
	recipients := map[string]int{}
	manifest := [][]string{{"filename", "source_id", "source_name", "source_sha256", "recipient_id", "name", "watermark_id", "key_id", "rendered_sha256", "quality_json"}}
	var original image.Image
	lastSource := ""
	for n, c := range j.Copies {
		if err = ctx.Err(); err != nil {
			return "", 0, err
		}
		var src distribution.JobSource
		for _, s := range j.Sources {
			if s.ID == c.SourceID {
				src = s
			}
		}
		copyID := ""
		if j.Mode == "dual" {
			copyID = c.ID
		}
		if j.Mode == "dual" || lastSource != c.SourceID {
			if err = m.checkInput(j, c.SourceID, copyID); err != nil {
				return "", 0, err
			}
			original, err = blind.Load(m.inputPath(j.ID, c.SourceID, copyID))
			if err != nil {
				return "", 0, err
			}
			lastSource = c.SourceID
		}
		marked, e := blind.Embed(original, c.ID, key, j.CreatedAt)
		if e != nil {
			return "", 0, e
		}
		verified, e := blind.Extract(marked, key)
		if e != nil {
			return "", 0, e
		}
		if verified.Watermark != c.ID || verified.Timestamp != j.CreatedAt.Format("2006-01-02T15:04:05Z07:00") {
			return "", 0, blind.ErrInvalid
		}
		var rawSource image.Image
		if j.Mode == "dual" {
			rawSource, err = blind.Load(m.inputPath(j.ID, c.SourceID, ""))
			if err != nil {
				return "", 0, err
			}
		}
		policy := quality.Policy{}
		if j.QualityPolicy != nil {
			policy = *j.QualityPolicy
		}
		summary := quality.Compare(original, marked, rawSource, policy)
		if err = m.store.ObserveJobQuality(j.ID, c.ID, summary); err != nil {
			return "", 0, err
		}
		if !summary.Accepted {
			return "", 0, quality.ErrThreshold
		}
		qualityJSON, _ := json.Marshal(summary)
		if err = ctx.Err(); err != nil {
			return "", 0, err
		}
		if recipients[c.Recipient.ID] == 0 {
			recipients[c.Recipient.ID] = len(recipients) + 1
		}
		filename := fmt.Sprintf("recipient-%03d/%s-%s.png", recipients[c.Recipient.ID], src.ID, c.ID)
		h := &zip.FileHeader{Name: filename, Method: zip.Store}
		h.SetModTime(j.CreatedAt)
		entry, e := z.CreateHeader(h)
		if e != nil {
			return "", 0, e
		}
		if e = png.Encode(entry, marked); e != nil {
			return "", 0, e
		}
		manifest = append(manifest, []string{filename, src.ID, cell(src.Name), src.SHA256, cell(c.Recipient.ID), cell(c.Name), c.ID, j.KeyID, c.RenderedSHA256, string(qualityJSON)})
		if err = m.store.JobProgress(j.ID, n+1, "generating"); err != nil {
			return "", 0, err
		}
	}
	if err = m.store.JobProgress(j.ID, j.Total, "packing"); err != nil {
		return "", 0, err
	}
	entry, err := z.Create("manifest.csv")
	if err != nil {
		return "", 0, err
	}
	csvOut := csv.NewWriter(entry)
	if err = csvOut.WriteAll(manifest); err != nil {
		return "", 0, err
	}
	if err = z.Close(); err != nil {
		return "", 0, err
	}
	if err = f.Sync(); err != nil {
		return "", 0, err
	}
	if err = f.Close(); err != nil {
		return "", 0, err
	}
	closed = true
	if err = ctx.Err(); err != nil {
		return "", 0, err
	}
	if err = os.Rename(path, m.archivePath(j.ID)); err != nil {
		return "", 0, err
	}
	dir, e := os.Open(filepath.Dir(path))
	if e != nil {
		return "", 0, e
	}
	err = dir.Sync()
	_ = dir.Close()
	if err != nil {
		return "", 0, err
	}
	return fileHash(m.archivePath(j.ID))
}
