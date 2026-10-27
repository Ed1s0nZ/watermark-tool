package filetasks

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"watermark-tool/internal/filebundle"
)

type boundedWriter struct {
	ctx       context.Context
	out       io.Writer
	remaining int64
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, ErrLimit
	}
	n, err := w.out.Write(p)
	w.remaining -= int64(n)
	return n, err
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func digestFile(ctx context.Context, path string, limit int64) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return "", 0, ErrStore
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, ErrStore
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(contextReader{ctx, f}, limit+1))
	if err != nil {
		return "", 0, err
	}
	if n != info.Size() || n > limit {
		return "", 0, ErrStore
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
func csvCell(s string) string {
	if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}
func manifestCSV(j Job) []byte {
	var raw bytes.Buffer
	writer := csv.NewWriter(&raw)
	_ = writer.Write([]string{"source_id", "source_path", "source_sha256", "status", "attempts", "output_path", "algorithm", "output_sha256", "final_verified", "error"})
	for _, item := range j.Items {
		algorithm, hash, verified := "", "", "false"
		if item.Plan != nil {
			algorithm = item.Plan.Algorithm
		}
		if item.Result != nil {
			hash = item.Result.OutputSHA256
			if item.Result.FinalVerified {
				verified = "true"
			}
		}
		_ = writer.Write([]string{item.Source.ID, csvCell(item.Source.RelativePath), item.Source.SHA256, item.Status, strconv.Itoa(len(item.Attempts)), csvCell("files/" + item.OutputPath), algorithm, hash, verified, item.Error})
	}
	writer.Flush()
	return raw.Bytes()
}
func zipBytes(writer *zip.Writer, name string, raw []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0600)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(raw)
	return err
}
func (m *Manager) buildArchive(ctx context.Context, id string) (err error) {
	j, err := m.Get(id)
	if err != nil {
		return err
	}
	if _, err = m.gate(j); err != nil {
		return err
	}
	file, err := os.CreateTemp(m.path("archives", ""), ".filezip-*.tmp")
	if err != nil {
		return ErrStore
	}
	defer os.Remove(file.Name())
	defer file.Close()
	writer := zip.NewWriter(&boundedWriter{ctx: ctx, out: file, remaining: MaxArchiveBytes})
	closed := false
	defer func() {
		if !closed {
			writer.Close()
		}
	}()
	for _, dir := range j.Directories {
		if _, err = filebundle.SafePath(dir); err != nil {
			return err
		}
		header := &zip.FileHeader{Name: "files/" + dir + "/", Method: zip.Store}
		header.SetMode(os.ModeDir | 0700)
		if _, err = writer.CreateHeader(header); err != nil {
			return err
		}
	}
	for _, item := range j.Items {
		if item.Status != "completed" {
			continue
		}
		if _, err = filebundle.SafePath(item.OutputPath); err != nil {
			return err
		}
		physical := filepath.Join(m.path("outputs", id), item.Source.ID)
		input, e := os.Open(physical)
		if e != nil {
			return ErrStore
		}
		header := &zip.FileHeader{Name: "files/" + item.OutputPath, Method: zip.Deflate}
		header.SetMode(0600)
		entry, e := writer.CreateHeader(header)
		hash := sha256.New()
		var n int64
		if e == nil {
			n, e = io.Copy(entry, io.LimitReader(contextReader{ctx, io.TeeReader(input, hash)}, MaxArchiveBytes+1))
		}
		input.Close()
		if e != nil {
			return e
		}
		if item.Result == nil || n != item.Result.OutputBytes || hex.EncodeToString(hash.Sum(nil)) != item.Result.OutputSHA256 {
			return ErrStore
		}
		info, e := os.Lstat(physical + ".report.json")
		if e != nil || !info.Mode().IsRegular() || info.Size() > MaxJobBytes {
			return ErrStore
		}
		report, e := os.ReadFile(physical + ".report.json")
		if e != nil {
			return ErrStore
		}
		reportHash := sha256.Sum256(report)
		if int64(len(report)) != item.ReportBytes || hex.EncodeToString(reportHash[:]) != item.ReportSHA256 {
			return ErrStore
		}
		var actual any
		if json.Unmarshal(report, &actual) != nil {
			return ErrStore
		}
		if err = zipBytes(writer, "reports/"+item.Source.ID+".json", report); err != nil {
			return err
		}
	}
	manifest, err := json.Marshal(j)
	if err != nil {
		return ErrStore
	}
	if err = zipBytes(writer, "manifest.json", manifest); err != nil {
		return err
	}
	if err = zipBytes(writer, "manifest.csv", manifestCSV(j)); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	closed = true
	if err = file.Sync(); err != nil {
		return ErrStore
	}
	if err = file.Close(); err != nil {
		return ErrStore
	}
	digest, n, err := digestFile(ctx, file.Name(), MaxArchiveBytes)
	if err != nil {
		return err
	}
	// Re-open the archive and read every entry/CRC, not just ZIP close success.
	archive, err := zip.OpenReader(file.Name())
	if err != nil {
		return ErrStore
	}
	for _, entry := range archive.File {
		reader, e := entry.Open()
		if e == nil {
			_, e = io.Copy(io.Discard, contextReader{ctx, reader})
			reader.Close()
		}
		if e != nil {
			archive.Close()
			return ErrStore
		}
	}
	archive.Close()
	m.mu.Lock()
	defer m.mu.Unlock()
	current, err := m.getLocked(id)
	if err != nil {
		return err
	}
	if current.Revision != j.Revision {
		return ErrState
	}
	if _, err = m.gate(current); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	destination := m.path("archives", id+".zip")
	if err = os.Rename(file.Name(), destination); err != nil {
		return ErrStore
	}
	parent, err := os.Open(filepath.Dir(destination))
	if err != nil {
		os.Remove(destination)
		return ErrStore
	}
	err = parent.Sync()
	parent.Close()
	if err != nil {
		os.Remove(destination)
		return ErrStore
	}
	current.ArchiveSHA256 = digest
	current.ArchiveBytes = n
	current.ArchiveAvailable = true
	if err = m.saveJobLocked(current); err != nil {
		os.Remove(destination)
		return err
	}
	return nil
}

// OpenArchive returns a verified descriptor only after current authority checks.
func (m *Manager) OpenArchive(ctx context.Context, id string) (*os.File, Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.getLocked(id)
	if err != nil {
		return nil, Job{}, err
	}
	if !j.ArchiveAvailable || !m.now().Before(j.ExpiresAt) {
		return nil, Job{}, ErrExpired
	}
	if _, err = m.gate(j); err != nil {
		return nil, Job{}, err
	}
	name := m.path("archives", id+".zip")
	info, e := os.Lstat(name)
	if e != nil || !info.Mode().IsRegular() {
		return nil, Job{}, ErrStore
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, Job{}, ErrStore
	}
	hash := sha256.New()
	n, e := io.Copy(hash, io.LimitReader(contextReader{ctx, f}, MaxArchiveBytes+1))
	if e != nil || n != j.ArchiveBytes || hex.EncodeToString(hash.Sum(nil)) != j.ArchiveSHA256 {
		f.Close()
		return nil, Job{}, ErrStore
	}
	if _, err = f.Seek(0, 0); err != nil {
		f.Close()
		return nil, Job{}, ErrStore
	}
	if _, err = m.gate(j); err != nil {
		f.Close()
		return nil, Job{}, err
	}
	return f, j, nil
}
