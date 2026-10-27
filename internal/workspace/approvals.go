package workspace

import (
	"encoding/json"
	"sort"
	"time"
	"watermark-tool/internal/distribution"
)

const MaxApprovals = 1000
const MaxApprovalChanges = 40

type ApprovalChange struct {
	Status string    `json:"status"`
	Actor  string    `json:"actor"`
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}
type Approval struct {
	TeamID    string           `json:"team_id"`
	JobID     string           `json:"job_id"`
	Operator  string           `json:"operator"`
	Requester string           `json:"requester"`
	Snapshot  string           `json:"snapshot"`
	Revision  uint64           `json:"revision"`
	Status    string           `json:"status"`
	Reviewer  string           `json:"reviewer"`
	Changes   []ApprovalChange `json:"changes"`
}

func approvalKey(team, job string) string { return team + "/" + job }
func cloneApproval(a Approval) Approval {
	raw, _ := json.Marshal(a)
	var result Approval
	_ = json.Unmarshal(raw, &result)
	return result
}
func validApprovalStatus(s string) bool {
	return s == "pending" || s == "approved" || s == "rejected" || s == "withdrawn"
}
func (s *Store) validateApprovals(d state) error {
	counts := map[string]int{}
	for key, a := range d.Approvals {
		if key != approvalKey(a.TeamID, a.JobID) || !teamPattern.MatchString(a.JobID) || !distribution.ValidDigest(a.Snapshot) || !validName(a.Operator, 256) || a.Operator != a.Requester || a.Revision == 0 || !validApprovalStatus(a.Status) || len(a.Changes) == 0 || len(a.Changes) > MaxApprovalChanges || a.Revision != uint64(len(a.Changes)) {
			return ErrInvalid
		}
		if _, ok := d.Teams[a.TeamID]; !ok {
			return ErrInvalid
		}
		counts[a.TeamID]++
		if counts[a.TeamID] > MaxApprovals {
			return ErrLimit
		}
		if a.Status == "approved" && (a.Reviewer == a.Requester || !validName(a.Reviewer, 256)) {
			return ErrInvalid
		}
		for _, c := range a.Changes {
			if !validApprovalStatus(c.Status) || !validName(c.Actor, 256) || c.At.IsZero() || (c.Reason != "" && !validName(c.Reason, 500)) {
				return ErrInvalid
			}
		}
		if a.Changes[len(a.Changes)-1].Status != a.Status {
			return ErrInvalid
		}
	}
	return nil
}
func (s *Store) SetApprovalRequired(actor, team string, revision uint64, required bool) (Team, error) {
	if team == PersonalTeam {
		return Team{}, ErrInvalid
	}
	var result Team
	err := s.change(actor, team, "team.approval", team, func(d *state) error {
		if !s.can(*d, actor, team, "manage") {
			return ErrDenied
		}
		t := d.Teams[team]
		if t.Revision != revision {
			return ErrConflict
		}
		t.RequireApproval = required
		t.Revision++
		t.UpdatedAt = now()
		d.Teams[team] = t
		result = cloneTeam(t)
		return nil
	})
	return result, err
}
func (s *Store) RequestApproval(actor, team string, j distribution.Job, revision uint64) (Approval, error) {
	snapshot, err := JobApprovalDigest(team, j)
	if err != nil {
		return Approval{}, err
	}
	if actor != j.Operator || j.Generated || (j.Status != "preparing" && j.Status != "failed" && j.Status != "cancelled" && j.Status != "interrupted") {
		return Approval{}, ErrDenied
	}
	for _, source := range j.Sources {
		if !source.Uploaded {
			return Approval{}, ErrConflict
		}
	}
	if j.Mode == "dual" {
		for _, copy := range j.Copies {
			if !copy.Uploaded {
				return Approval{}, ErrConflict
			}
		}
	}
	var result Approval
	err = s.change(actor, team, "approval.request", j.ID, func(d *state) error {
		if !s.can(*d, actor, team, "produce") {
			return ErrDenied
		}
		if d.Approvals == nil {
			d.Approvals = map[string]Approval{}
		}
		key := approvalKey(team, j.ID)
		current := d.Approvals[key]
		if current.Revision != revision {
			return ErrConflict
		}
		if len(current.Changes) >= MaxApprovalChanges-1 {
			return ErrLimit
		}
		result = Approval{TeamID: team, JobID: j.ID, Operator: j.Operator, Requester: actor, Snapshot: snapshot, Revision: revision + 1, Status: "pending", Changes: append(current.Changes, ApprovalChange{Status: "pending", Actor: actor, At: now()})}
		d.Approvals[key] = result
		return nil
	})
	if err != nil {
		return Approval{}, err
	}
	return cloneApproval(result), nil
}
func (s *Store) DecideApproval(actor, team string, j distribution.Job, revision uint64, status, reason string, receipts ...string) (Approval, error) {
	if (status != "approved" && status != "rejected" && status != "withdrawn") || (reason != "" && !validName(reason, 500)) {
		return Approval{}, ErrInvalid
	}
	snapshot, err := JobApprovalDigest(team, j)
	if err != nil {
		return Approval{}, err
	}
	var result Approval
	err = s.change(actor, team, "approval."+status, j.ID, func(d *state) error {
		a, ok := d.Approvals[approvalKey(team, j.ID)]
		if !ok {
			return ErrMissing
		}
		if a.Revision != revision {
			return ErrConflict
		}
		if status == "withdrawn" {
			if !(s.can(*d, actor, team, "produce") && (actor == a.Requester || s.can(*d, actor, team, "manage"))) && !(actor == a.Reviewer && s.can(*d, actor, team, "approve")) {
				return ErrDenied
			}
		} else {
			if !s.can(*d, actor, team, "approve") || actor == a.Requester || actor == a.Operator {
				return ErrDenied
			}
			if a.Status != "pending" || a.Snapshot != snapshot || !s.can(*d, a.Requester, team, "produce") {
				return ErrConflict
			}
		}
		if status == "approved" && (len(receipts) != 1 || !s.validReviewReceipt(actor, a, receipts[0])) {
			return ErrConflict
		}
		if len(a.Changes) >= MaxApprovalChanges || status != "withdrawn" && len(a.Changes) >= MaxApprovalChanges-1 {
			return ErrLimit
		}
		a.Revision++
		a.Status = status
		a.Reviewer = ""
		if status == "approved" {
			a.Reviewer = actor
		}
		a.Changes = append(a.Changes, ApprovalChange{Status: status, Actor: actor, Reason: reason, At: now()})
		d.Approvals[approvalKey(team, j.ID)] = a
		result = a
		return nil
	})
	if err != nil {
		return Approval{}, err
	}
	return cloneApproval(result), nil
}
func (s *Store) Approvals(actor, team string) ([]Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return nil, ErrUnavailable
	}
	if !s.can(s.data, actor, team, "read") {
		return nil, ErrDenied
	}
	result := []Approval{}
	for _, a := range s.data.Approvals {
		if a.TeamID == team {
			result = append(result, cloneApproval(a))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Changes[len(result[i].Changes)-1].At.After(result[j].Changes[len(result[j].Changes)-1].At)
	})
	return result, nil
}
func (s *Store) CheckJobApproval(team string, j distribution.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return ErrUnavailable
	}
	if !s.can(s.data, j.Operator, team, "produce") {
		return ErrDenied
	}
	if !s.data.Teams[team].RequireApproval {
		return nil
	}
	snapshot, err := JobApprovalDigest(team, j)
	if err != nil {
		return err
	}
	a, ok := s.data.Approvals[approvalKey(team, j.ID)]
	if !ok || a.Status != "approved" || a.Snapshot != snapshot || a.Operator != j.Operator || a.Requester != j.Operator || a.Reviewer == j.Operator || !s.can(s.data, a.Reviewer, team, "approve") {
		return ErrApproval
	}
	return nil
}
func (s *Store) ApprovalFor(actor, team, job string) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return nil, ErrUnavailable
	}
	if !s.can(s.data, actor, team, "read") {
		return nil, ErrDenied
	}
	a, ok := s.data.Approvals[approvalKey(team, job)]
	if !ok {
		return nil, nil
	}
	copy := cloneApproval(a)
	return &copy, nil
}
