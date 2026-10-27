package filetasks

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/fileprocess"
	"watermark-tool/internal/output"
	"watermark-tool/internal/pdfmark"
	"watermark-tool/internal/quality"
	"watermark-tool/internal/watermark"
)

func errorCode(err error) string {
	for _, known := range []error{fileprocess.ErrUnsupported, fileprocess.ErrOptions, fileprocess.ErrSource, fileprocess.ErrStructure, fileprocess.ErrVerification, fileprocess.ErrIO, blind.ErrText, blind.ErrCapacity, blind.ErrImage, blind.ErrKey, output.ErrVerification, output.ErrOptions, quality.ErrThreshold, quality.ErrPolicy, pdfmark.ErrInvalid, pdfmark.ErrEncrypted, pdfmark.ErrSigned, pdfmark.ErrLimit, pdfmark.ErrSelection, pdfmark.ErrText, pdfmark.ErrMarked, pdfmark.ErrVerification, pdfmark.ErrIO, pdfmark.ErrChanged, filebundle.ErrArchive, filebundle.ErrPath, filebundle.ErrLimit, filebundle.ErrLink, filebundle.ErrChecksum, filebundle.ErrIO, watermark.ErrSignedCompatibility, watermark.ErrXMLMetadata, watermark.ErrCompatibilityOutput, ErrLimit, ErrExpired, ErrDenied} {
		if errors.Is(err, known) || err.Error() == known.Error() {
			return known.Error()
		}
	}
	if cancelled(err) {
		return "cancelled"
	}
	switch err.Error() {
	case "approval_required", "workspace_denied", "workspace_unavailable":
		return err.Error()
	}
	return "file_processing_failed"
}
func compact(result fileprocess.Result) fileprocess.Result {
	r := clone(result)
	r.Plan.Pages = nil
	if r.PDF != nil {
		r.PDF.Pages = nil
	}
	return r
}
func (m *Manager) dispatch(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing || m.poisoned || m.running[id] != nil {
		return
	}
	j, err := m.getLocked(id)
	if err != nil || j.Status != "queued" {
		return
	}
	select {
	case m.options.Slots <- struct{}{}:
	default:
		return
	}
	usage, err := m.usageLocked()
	if err != nil || usage+m.reserved+3*MaxArchiveBytes > MaxDiskBytes {
		<-m.options.Slots
		return
	}
	if _, err = m.gate(j); err != nil {
		j = failItems(j, errorCode(err))
		j.Status = "failed"
		j.FinishedAt = m.now()
		j.ExpiresAt = m.now().Add(m.options.ArchiveTTL)
		j.Error = errorCode(err)
		_ = m.saveJobLocked(j)
		<-m.options.Slots
		return
	}
	if !m.now().Before(j.InputExpiresAt) {
		j = failItems(j, ErrExpired.Error())
		j.Status = "failed"
		j.FinishedAt = m.now()
		j.ExpiresAt = m.now().Add(m.options.ArchiveTTL)
		j.Error = ErrExpired.Error()
		_ = m.saveJobLocked(j)
		<-m.options.Slots
		return
	}
	j.Status = "running"
	j.StartedAt = m.now()
	j.ArchiveAvailable = false
	j.Error = ""
	if err = m.saveJobLocked(j); err != nil {
		<-m.options.Slots
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.running[id] = cancel
	m.reserved += 3 * MaxArchiveBytes
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			cancel()
			m.mu.Lock()
			delete(m.running, id)
			m.reserved -= 3 * MaxArchiveBytes
			m.mu.Unlock()
			<-m.options.Slots
		}()
		m.run(ctx, id)
	}()
}
func (m *Manager) run(ctx context.Context, id string) {
	if err := privateDir(m.path("outputs", id)); err != nil {
		m.finishFault(id, err)
		return
	}
	// A retry invalidates its old ZIP but preserves all successful item bytes.
	_ = os.Remove(m.path("archives", id+".zip"))
	for index := 0; ; index++ {
		m.mu.Lock()
		j, err := m.getLocked(id)
		if err != nil || index >= len(j.Items) {
			m.mu.Unlock()
			break
		}
		item := j.Items[index]
		if item.Status != "pending" {
			m.mu.Unlock()
			continue
		}
		if ctx.Err() != nil {
			m.mu.Unlock()
			break
		}
		key, err := m.gate(j)
		if err != nil {
			m.mu.Unlock()
			m.finishFault(id, err)
			return
		}
		item.Status = "running"
		item.Attempts = append(item.Attempts, Attempt{Number: len(item.Attempts) + 1, Status: "running", StartedAt: m.now(), At: m.now()})
		j.Items[index] = item
		if err = m.saveJobLocked(j); err != nil {
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
		physical := filepath.Join(m.path("outputs", id), item.Source.ID)
		_ = os.Remove(physical)
		_ = os.Remove(physical + ".report.json")
		var result fileprocess.Result
		var reportHash string
		var reportBytes int64
		if item.PlanError != "" {
			err = errors.New(item.PlanError)
		} else {
			result, err = fileprocess.Execute(ctx, m.source(id, item.Source), physical, fileprocess.Identity{DocumentID: item.Source.ID, KeyID: j.KeyID, CreatedAt: j.CreatedAt}, item.Options, key)
		}
		clear(key)
		if err == nil {
			raw, e := json.Marshal(result)
			if e != nil || len(raw) > MaxJobBytes {
				err = ErrLimit
			} else {
				err = atomicWrite(physical+".report.json", raw)
				if err == nil {
					reportHash, reportBytes, err = digestFile(ctx, physical+".report.json", MaxJobBytes)
				}
			}
		}
		m.mu.Lock()
		current, e := m.getLocked(id)
		if e != nil {
			m.mu.Unlock()
			return
		}
		if err == nil {
			_, err = m.gate(current)
		}
		if err == nil {
			usage, e := directoryBytes(m.path("outputs", id))
			if e != nil || usage > MaxArchiveBytes-(4<<20) {
				err = ErrLimit
			}
		}
		item = current.Items[index]
		item.Error = ""
		item.ReportSHA256 = ""
		item.ReportBytes = 0
		if err != nil {
			os.Remove(physical)
			os.Remove(physical + ".report.json")
			item.Status = "failed"
			if cancelled(err) {
				item.Status = "cancelled"
				if m.ctx.Err() != nil {
					item.Status = "interrupted"
				}
			}
			item.Error = errorCode(err)
			item.Result = nil
		} else {
			r := compact(result)
			raw, _ := json.Marshal(r)
			if len(raw) > 4096 {
				os.Remove(physical)
				os.Remove(physical + ".report.json")
				item.Status = "failed"
				item.Error = ErrLimit.Error()
			} else {
				item.Result = &r
				item.ReportSHA256 = reportHash
				item.ReportBytes = reportBytes
				item.Status = "completed"
			}
		}
		last := len(item.Attempts) - 1
		item.Attempts[last].Status = item.Status
		item.Attempts[last].Error = item.Error
		item.Attempts[last].At = m.now()
		current.Items[index] = item
		e = m.saveJobLocked(current)
		m.mu.Unlock()
		if e != nil {
			return
		}
		if ctx.Err() != nil {
			break
		}
	}
	m.mu.Lock()
	j, err := m.getLocked(id)
	if err != nil {
		m.mu.Unlock()
		return
	}
	done := 0
	for i := range j.Items {
		if j.Items[i].Status == "completed" {
			done++
		}
		if j.Items[i].Status == "pending" || j.Items[i].Status == "running" {
			j.Items[i].Status = "cancelled"
			if m.ctx.Err() != nil {
				j.Items[i].Status = "interrupted"
			}
		}
	}
	j.Error = ""
	switch {
	case m.ctx.Err() != nil:
		j.Status = "interrupted"
	case ctx.Err() != nil:
		j.Status = "cancelled"
	case done == len(j.Items):
		j.Status = "completed"
	case done > 0:
		j.Status = "partial"
	default:
		j.Status = "failed"
	}
	j.FinishedAt = m.now()
	j.ExpiresAt = m.now().Add(m.options.ArchiveTTL)
	err = m.saveJobLocked(j)
	m.mu.Unlock()
	if err != nil || m.ctx.Err() != nil {
		return
	}
	// Finishing a cancellation may publish only already-verified successes + report.
	archiveCtx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()
	if err = m.buildArchive(archiveCtx, id); err != nil {
		m.finishFault(id, err)
	}
}
func (m *Manager) finishFault(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, e := m.getLocked(id)
	if e != nil {
		return
	}
	j.Error = errorCode(err)
	j.ArchiveAvailable = false
	if j.Status == "running" || j.Status == "queued" {
		j.Status = "failed"
	}
	j = failItems(j, j.Error)
	j.FinishedAt = m.now()
	j.ExpiresAt = m.now().Add(m.options.ArchiveTTL)
	_ = m.saveJobLocked(j)
}

func failItems(j Job, code string) Job {
	for i := range j.Items {
		item := &j.Items[i]
		if item.Status == "pending" || item.Status == "running" {
			item.Status = "failed"
			item.Error = code
			if n := len(item.Attempts); n > 0 && item.Attempts[n-1].Status == "running" {
				item.Attempts[n-1].Status = "failed"
				item.Attempts[n-1].Error = code
			}
		}
	}
	return j
}
