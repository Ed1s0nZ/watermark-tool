package distribution

import (
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/quality"
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ValidDigest(s string) bool { return digestPattern.MatchString(s) }
func finiteNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	case int:
		return float64(n), true
	}
	return 0, false
}
func ValidStyle(s map[string]any) bool {
	if len(s) != 6 {
		return false
	}
	mode, _ := s["mode"].(string)
	position, _ := s["position"].(string)
	color, _ := s["color"].(string)
	size, a := finiteNumber(s["size"])
	opacity, b := finiteNumber(s["opacity"])
	angle, c := finiteNumber(s["angle"])
	return (mode == "single" || mode == "tile") && strings.Contains("|topLeft|topRight|center|bottomLeft|bottomRight|", "|"+position+"|") && position != "" && regexp.MustCompile(`^#[a-fA-F0-9]{6}$`).MatchString(color) && a && size >= 12 && size <= 96 && b && opacity >= 0.05 && opacity <= 1 && c && angle >= -90 && angle <= 90
}
func VisibleText(template string, r Recipient) (string, error) {
	probe := strings.NewReplacer("{name}", "", "{recipient_id}", "").Replace(template)
	if strings.ContainsAny(probe, "{}") {
		return "", ErrTemplate
	}
	text := strings.NewReplacer("{name}", r.Name, "{recipient_id}", r.ID).Replace(template)
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 80 {
		return "", ErrTemplate
	}
	for _, c := range text {
		if unicode.IsControl(c) || unicode.Is(unicode.Cf, c) {
			return "", ErrTemplate
		}
	}
	return text, nil
}
func (s *Store) jobsSnapshot() []Job {
	out := make([]Job, len(s.jobs))
	for i, j := range s.jobs {
		out[i] = cloneJob(j)
	}
	return out
}
func pendingJob(status string) bool {
	return status == "preparing" || status == "queued" || status == "processing" || status == "cancelling"
}
func (s *Store) NewJob(request, actor string, sources []JobSource, recipients []Recipient, mode, text string, style map[string]any, policies ...quality.Policy) (Job, error) {
	var policy *quality.Policy
	if len(policies) > 1 {
		return Job{}, quality.ErrPolicy
	}
	if len(policies) == 1 {
		if !policies[0].Valid() {
			return Job{}, quality.ErrPolicy
		}
		policy = quality.Clone(&policies[0])
	}
	if len(request) < 16 || len(request) > 128 || actor == "" || len(actor) > 256 || !utf8.ValidString(actor) || len(sources) < 1 || len(sources) > MaxSources || len(recipients) < 1 || len(recipients) > MaxRecipients || (mode != "blind" && mode != "dual") {
		return Job{}, ErrRecipients
	}
	names := map[string]bool{}
	clean := make([]JobSource, len(sources))
	for i, src := range sources {
		if src.Name == "" || len(src.Name) > 255 || !utf8.ValidString(src.Name) || !ValidDigest(src.SHA256) || names[src.SHA256] || src.Width <= 0 || src.Height <= 0 || src.Width > blind.MaxPixels/src.Height || (src.Width/8)*(src.Height/8) < 1536 {
			return Job{}, ErrRecipients
		}
		names[src.SHA256] = true
		clean[i] = JobSource{Name: src.Name, SHA256: src.SHA256, Width: src.Width, Height: src.Height}
	}
	ids := map[string]bool{}
	for _, r := range recipients {
		if r.ID == "" || r.Name == "" || len(r.ID) > 64 || len(r.Name) > 128 || !utf8.ValidString(r.ID) || !utf8.ValidString(r.Name) || ids[r.ID] {
			return Job{}, ErrRecipients
		}
		ids[r.ID] = true
		if mode == "dual" {
			if _, err := VisibleText(text, r); err != nil {
				return Job{}, err
			}
		}
	}
	if mode == "dual" && !ValidStyle(style) {
		return Job{}, ErrTemplate
	}
	if mode == "blind" {
		text = ""
		style = nil
	}
	fingerprintPolicy := quality.Clone(policy)
	if fingerprintPolicy != nil && fingerprintPolicy.MinimumPSNR == nil {
		fingerprintPolicy = nil
	}
	raw, err := json.Marshal(struct {
		Actor      string
		Sources    []JobSource
		Recipients []Recipient
		Mode, Text string
		Style      map[string]any
		Quality    *quality.Policy `json:",omitempty"`
	}{actor, clean, recipients, mode, text, style, fingerprintPolicy})
	if err != nil {
		return Job{}, ErrRecipients
	}
	fp, rh := Digest(raw), Digest([]byte(request))
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Job{}, ErrStore
	}
	for _, j := range s.jobs {
		if j.RequestHash == rh {
			if j.Fingerprint != fp {
				return Job{}, ErrConflict
			}
			return publicJob(j), nil
		}
	}
	if s.noNewWrites() || len(s.jobs) >= MaxJobs {
		return Job{}, ErrLimit
	}
	pending := 0
	for _, j := range s.jobs {
		if pendingJob(j.Status) {
			pending++
		}
	}
	if pending >= MaxPendingJobs {
		return Job{}, ErrBusy
	}
	keyID := ""
	for _, k := range s.keys.Keys {
		if k.State == "active" {
			keyID = k.ID
		}
	}
	if keyID == "" {
		return Job{}, ErrState
	}
	id, err := randomID(16)
	if err != nil {
		return Job{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	job := Job{QualityPolicy: policy, ID: id, Operator: actor, LastActor: actor, CreatedAt: now, UpdatedAt: now, KeyID: keyID, Algorithm: blind.Algorithm, Mode: mode, TemplateText: text, Style: style, Status: "preparing", Phase: "upload", Sources: clean, RequestHash: rh, Fingerprint: fp, Total: len(clean) * len(recipients)}
	for i := range job.Sources {
		sid, e := randomID(16)
		if e != nil {
			return Job{}, e
		}
		job.Sources[i].ID = sid
		for _, r := range recipients {
			cid, e := randomID(16)
			if e != nil {
				return Job{}, e
			}
			job.Copies = append(job.Copies, JobCopy{Copy: Copy{cid, r}, SourceID: sid})
		}
	}
	if err = rotateUpload(&job); err != nil {
		return Job{}, err
	}
	job = cloneJob(job)
	next := s.jobsSnapshot()
	next = append(next, job)
	if err = s.commitPlatform(s.data, s.keys, next, NewEvent(actor, "task.create", id, "preparing", "")); err != nil {
		return Job{}, err
	}
	return publicJob(job), nil
}
func (s *Store) Job(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Job{}, ErrStore
	}
	for _, j := range s.jobs {
		if j.ID == id {
			return publicJob(j), nil
		}
	}
	return Job{}, ErrMissing
}
func (s *Store) Jobs(q string, page, size int) ([]Job, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return nil, 0, ErrStore
	}
	if !ValidPage(page, size) || len(q) > 256 {
		return nil, 0, ErrRecipients
	}
	q = strings.ToLower(q)
	out := []Job{}
	for i := len(s.jobs) - 1; i >= 0; i-- {
		j := s.jobs[i]
		found := matches(q, j.ID, j.Status, j.Operator)
		for _, src := range j.Sources {
			found = found || matches(q, src.ID, src.Name, src.SHA256)
		}
		for _, c := range j.Copies {
			found = found || matches(q, c.ID, c.Name, c.Recipient.ID)
		}
		if found {
			out = append(out, publicJob(j))
		}
	}
	total := len(out)
	a, b := pageBounds(total, page, size)
	return out[a:b], total, nil
}
