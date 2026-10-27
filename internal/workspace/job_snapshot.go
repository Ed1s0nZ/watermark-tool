package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"watermark-tool/internal/distribution"
)

// JobApprovalDigest excludes transient upload, queue, token and measured-output state.
// All inputs affecting the delivered watermark, recipient mapping and encoding remain bound.
func JobApprovalDigest(team string, j distribution.Job) (string, error) {
	if !teamPattern.MatchString(j.ID) || j.Operator == "" || j.CreatedAt.IsZero() || len(j.Sources) == 0 || len(j.Copies) == 0 {
		return "", ErrInvalid
	}
	type source struct {
		ID, Name, SHA256 string
		Width, Height    int
		Bytes            int64
	}
	type copy struct {
		ID, SourceID, RecipientID, Name, RenderedSHA256 string
		InputBytes                                      int64
	}
	sources := []source{}
	copies := []copy{}
	for _, s := range j.Sources {
		sources = append(sources, source{s.ID, s.Name, s.SHA256, s.Width, s.Height, s.Bytes})
	}
	for _, c := range j.Copies {
		copies = append(copies, copy{c.ID, c.SourceID, c.Recipient.ID, c.Name, c.RenderedSHA256, c.InputBytes})
	}
	value := map[string]any{"schema_version": 1, "team": team, "id": j.ID, "operator": j.Operator, "created_at": j.CreatedAt, "key_id": j.KeyID, "algorithm": j.Algorithm, "mode": j.Mode, "template": j.TemplateText, "style": j.Style, "quality": j.QualityPolicy, "artwork": j.Artwork, "output": j.Output, "sources": sources, "copies": copies}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", ErrInvalid
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
