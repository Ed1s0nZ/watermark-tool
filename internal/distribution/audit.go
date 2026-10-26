package distribution

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type AuditEvent struct {
	Sequence    uint64    `json:"sequence"`
	ID          string    `json:"id"`
	At          time.Time `json:"at"`
	Actor       string    `json:"actor"`
	Action      string    `json:"action"`
	Target      string    `json:"target,omitempty"`
	Correlation string    `json:"correlation"`
	Outcome     string    `json:"outcome"`
	Code        string    `json:"code,omitempty"`
}

var auditID = regexp.MustCompile(`^[a-f0-9]{24,32}$`)
var auditWord = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

func NewEvent(actor, action, target, outcome, code string) AuditEvent {
	id, _ := randomID(16)
	if actor == "" {
		actor = "system"
	}
	if target != "platform" && target != "legacy-env" && !auditID.MatchString(target) {
		target = ""
	}
	if code != "" && !auditWord.MatchString(code) {
		code = "request_error"
	}
	return AuditEvent{ID: id, At: time.Now().UTC(), Actor: actor, Action: action, Target: target, Correlation: id, Outcome: outcome, Code: code}
}
func validateEvent(e AuditEvent) error {
	if !auditID.MatchString(e.ID) || !auditID.MatchString(e.Correlation) || e.At.IsZero() || e.Actor == "" || len(e.Actor) > 256 || !utf8.ValidString(e.Actor) || !auditWord.MatchString(e.Action) || !auditWord.MatchString(e.Outcome) || e.Code != "" && !auditWord.MatchString(e.Code) {
		return ErrRecipients
	}
	if e.Target != "" && e.Target != "platform" && e.Target != "legacy-env" && !auditID.MatchString(e.Target) {
		return ErrRecipients
	}
	return nil
}
func (s *Store) Audit(actor, action, target, outcome, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.noNewWrites() {
		return ErrLimit
	}
	return s.commitPlatform(s.data, s.keys, s.jobs, NewEvent(actor, action, target, outcome, code))
}

type AuditFilter struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Actor   string `json:"actor"`
	Action  string `json:"action"`
	Outcome string `json:"outcome"`
	Page    int    `json:"page"`
	Size    int    `json:"page_size"`
	Format  string `json:"format,omitempty"`
}

func (s *Store) AuditSearch(f AuditFilter, export bool) ([]AuditEvent, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !ValidPage(f.Page, f.Size) || len(f.Actor) > 256 || len(f.Action) > 64 || len(f.Outcome) > 64 {
		return nil, 0, ErrRecipients
	}
	var from, to time.Time
	var err error
	if f.From != "" {
		from, err = time.Parse(time.RFC3339, f.From)
		if err != nil {
			return nil, 0, ErrRecipients
		}
	}
	if f.To != "" {
		to, err = time.Parse(time.RFC3339, f.To)
		if err != nil {
			return nil, 0, ErrRecipients
		}
	}
	if !to.IsZero() && !from.IsZero() && to.Before(from) {
		return nil, 0, ErrRecipients
	}
	out := []AuditEvent{}
	for i := len(s.audit) - 1; i >= 0; i-- {
		e := s.audit[i]
		if !from.IsZero() && e.At.Before(from) || !to.IsZero() && e.At.After(to) || f.Actor != "" && e.Actor != f.Actor || f.Action != "" && e.Action != f.Action || f.Outcome != "" && e.Outcome != f.Outcome {
			continue
		}
		out = append(out, e)
	}
	total := len(out)
	if export {
		if total > 10000 {
			return nil, total, ErrLimit
		}
		return out, total, nil
	}
	a, b := pageBounds(total, f.Page, f.Size)
	return out[a:b], total, nil
}
func (s *Store) AuditDegraded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.poisoned || s.noNewWrites()
}
func (s *Store) batchEvent(next state) AuditEvent {
	for i, b := range next.Batches {
		if i >= len(s.data.Batches) {
			return NewEvent(b.Operator, "batch.create", b.ID, "processing", "")
		}
		old := s.data.Batches[i]
		if b.Attempts != old.Attempts {
			return NewEvent(b.Operator, "batch.retry", b.ID, "processing", "")
		}
		if b.Status != old.Status || b.Error != old.Error {
			return NewEvent(b.Operator, "batch.finish", b.ID, b.Status, strings.ToLower(b.Error))
		}
	}
	return NewEvent("system", "storage.update", "platform", "completed", "")
}
