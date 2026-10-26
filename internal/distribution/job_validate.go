package distribution

import (
	"unicode/utf8"
	"watermark-tool/internal/blind"
)

func (s *Store) validateJobs() error {
	ids := map[string]bool{}
	requests := map[string]bool{}
	keys := map[string]bool{}
	copies := map[string]bool{}
	for _, k := range s.keys.Keys {
		keys[k.ID] = true
	}
	for _, b := range s.data.Batches {
		for _, c := range b.Copies {
			copies[c.ID] = true
		}
	}
	for _, j := range s.jobs {
		if len(j.UploadToken) != 32 || !auditID.MatchString(j.UploadToken) || !auditID.MatchString(j.ID) || len(j.ID) != 32 || ids[j.ID] || !ValidDigest(j.RequestHash) || requests[j.RequestHash] || !ValidDigest(j.Fingerprint) || !keys[j.KeyID] || j.Algorithm != blind.Algorithm || j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() || j.Operator == "" || len(j.Operator) > 256 || !utf8.ValidString(j.Operator) || j.LastActor == "" || len(j.LastActor) > 256 || !utf8.ValidString(j.LastActor) || len(j.Sources) < 1 || len(j.Sources) > MaxSources || len(j.Copies) < 1 || len(j.Copies) > 100 || j.Total != len(j.Copies) || j.Done < 0 || j.Done > j.Total || j.Attempts < 0 {
			return ErrStore
		}
		ids[j.ID] = true
		requests[j.RequestHash] = true
		switch j.Status {
		case "preparing", "queued", "processing", "cancelling", "completed", "failed", "cancelled", "interrupted":
		default:
			return ErrStore
		}
		if j.Mode != "blind" && j.Mode != "dual" || j.Mode == "dual" && !ValidStyle(j.Style) {
			return ErrStore
		}
		if j.Generated != (j.Status == "completed") {
			return ErrStore
		}
		if j.Generated && (!ValidDigest(j.ArchiveSHA256) || j.ExpiresAt.IsZero() || j.ArchiveBytes <= 0 || j.ArchiveBytes > MaxArchiveBytes) {
			return ErrStore
		}
		sourceIDs := map[string]bool{}
		sourceHashes := map[string]bool{}
		var original, total int64
		for _, src := range j.Sources {
			if len(src.ID) != 32 || !auditID.MatchString(src.ID) || sourceIDs[src.ID] || sourceHashes[src.SHA256] || !ValidDigest(src.SHA256) || src.Name == "" || len(src.Name) > 255 || !utf8.ValidString(src.Name) || src.Width <= 0 || src.Height <= 0 || src.Width > blind.MaxPixels/src.Height || src.Bytes < 0 || src.Bytes > 50<<20 || src.Uploaded && src.Bytes == 0 {
				return ErrStore
			}
			sourceIDs[src.ID] = true
			sourceHashes[src.SHA256] = true
			original += src.Bytes
		}
		total = original
		matrix := map[string]bool{}
		recipients := map[string]string{}
		for _, c := range j.Copies {
			pair := c.SourceID + "/" + c.Recipient.ID
			if len(c.ID) != 32 || !auditID.MatchString(c.ID) || copies[c.ID] || !sourceIDs[c.SourceID] || matrix[pair] || c.Recipient.ID == "" || len(c.Recipient.ID) > 64 || !utf8.ValidString(c.Recipient.ID) || c.Name == "" || len(c.Name) > 128 || !utf8.ValidString(c.Name) || c.RenderedSHA256 != "" && !ValidDigest(c.RenderedSHA256) || c.InputBytes < 0 || c.InputBytes > 50<<20 || c.Uploaded && (j.Mode != "dual" || c.RenderedSHA256 == "" || c.InputBytes == 0) {
				return ErrStore
			}
			if old, found := recipients[c.Recipient.ID]; found && old != c.Name {
				return ErrStore
			}
			recipients[c.Recipient.ID] = c.Name
			matrix[pair] = true
			copies[c.ID] = true
			total += c.InputBytes
			if j.Mode == "dual" {
				if _, err := VisibleText(j.TemplateText, c.Recipient); err != nil {
					return ErrStore
				}
			}
		}
		if len(recipients) > MaxRecipients || len(j.Copies) != len(recipients)*len(j.Sources) || original > MaxSourceBytes || total > MaxJobInputBytes {
			return ErrStore
		}
	}
	return nil
}
