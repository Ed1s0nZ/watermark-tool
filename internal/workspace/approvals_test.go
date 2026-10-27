package workspace

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
	"watermark-tool/internal/distribution"
)

func approvalJob() distribution.Job {
	return distribution.Job{ID: strings.Repeat("a", 32), Operator: "alice", CreatedAt: time.Now().UTC(), KeyID: "key", Mode: "blind", Algorithm: "dct-v1", Status: "preparing", Sources: []distribution.JobSource{{ID: strings.Repeat("b", 32), Name: "original.png", SHA256: strings.Repeat("c", 64), Width: 512, Height: 512, Bytes: 123, Uploaded: true}}, Copies: []distribution.JobCopy{{Copy: distribution.Copy{ID: strings.Repeat("d", 32), Recipient: distribution.Recipient{ID: "001", Name: "Alice"}}, SourceID: strings.Repeat("b", 32)}}}
}
func TestApprovalSnapshotRevocationAndRestart(t *testing.T) {
	s, dir, key := testStore(t)
	s.CreateUser("admin", "alice", "alice-password-123")
	s.CreateUser("admin", "bob", "bob-password-123")
	team, _ := s.CreateTeam("admin", "Review")
	team, _ = s.SetMember("admin", team.ID, "alice", RoleOwner, team.Revision)
	team, _ = s.SetMember("admin", team.ID, "bob", RoleReviewer, team.Revision)
	team, err := s.SetApprovalRequired("admin", team.ID, team.Revision, true)
	if err != nil {
		t.Fatal(err)
	}
	j := approvalJob()
	if err = s.CheckJobApproval(team.ID, j); !errors.Is(err, ErrApproval) {
		t.Fatal(err)
	}
	a, err := s.RequestApproval("alice", team.ID, j, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideApproval("alice", team.ID, j, a.Revision, "approved", ""); !errors.Is(err, ErrDenied) {
		t.Fatal("self approval", err)
	}
	if _, err = s.DecideApproval("admin", PersonalTeam, j, a.Revision, "approved", ""); !errors.Is(err, ErrMissing) {
		t.Fatal("cross team", err)
	}
	receipt, err := s.ReviewReceipt("bob", team.ID, j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideApproval("bob", team.ID, j, a.Revision, "approved", "", "forged"); !errors.Is(err, ErrConflict) {
		t.Fatal("forged receipt accepted", err)
	}
	if _, err = s.DecideApproval("admin", team.ID, j, a.Revision, "approved", "", receipt); !errors.Is(err, ErrConflict) {
		t.Fatal("receipt crossed reviewer identity", err)
	}
	expires := strconv.FormatInt(now().Add(-time.Minute).Unix(), 10)
	expired := expires + "." + s.reviewMAC("bob", a, expires)
	if _, err = s.DecideApproval("bob", team.ID, j, a.Revision, "approved", "", expired); !errors.Is(err, ErrConflict) {
		t.Fatal("expired receipt accepted", err)
	}
	a, err = s.DecideApproval("bob", team.ID, j, a.Revision, "approved", "Reviewed uploaded inputs", receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CheckJobApproval(team.ID, j); err != nil {
		t.Fatal(err)
	}
	stable, _ := JobApprovalDigest(team.ID, j)
	j.UploadToken = "new token"
	j.Status = "completed"
	j.Sources[0].Uploaded = false
	j.Generated = true
	j.Done = 1
	j.Attempts = 3
	same, _ := JobApprovalDigest(team.ID, j)
	if same != stable {
		t.Fatal("runtime changes invalidated immutable snapshot")
	}
	j.Copies[0].Name = "Different"
	if err = s.CheckJobApproval(team.ID, j); !errors.Is(err, ErrApproval) {
		t.Fatal("changed recipient accepted", err)
	}
	j.Copies[0].Name = "Alice"
	team, err = s.SetMember("admin", team.ID, "bob", RoleOperator, team.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CheckJobApproval(team.ID, j); !errors.Is(err, ErrApproval) {
		t.Fatal("revoked reviewer accepted", err)
	}
	team, _ = s.SetMember("admin", team.ID, "bob", RoleReviewer, team.Revision)
	a, err = s.DecideApproval("bob", team.ID, j, a.Revision, "withdrawn", "Withdraw my approval")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CheckJobApproval(team.ID, j); !errors.Is(err, ErrApproval) {
		t.Fatal("withdrawn accepted", err)
	}
	s.Close()
	s, err = Open(dir, key, "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	items, err := s.Approvals("bob", team.ID)
	if err != nil || len(items) != 1 || items[0].Status != "withdrawn" || len(items[0].Changes) != 3 {
		t.Fatal(items, err)
	}
	if _, err = s.SetApprovalRequired("admin", PersonalTeam, 1, true); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
func TestApprovalRejectsMissingInputsAndChangedDigest(t *testing.T) {
	s, _, _ := testStore(t)
	s.CreateUser("admin", "alice", "alice-password-123")
	team, _ := s.CreateTeam("admin", "Review")
	s.SetMember("admin", team.ID, "alice", RoleOperator, team.Revision)
	j := approvalJob()
	j.Sources[0].Uploaded = false
	if _, err := s.RequestApproval("alice", team.ID, j, 0); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	j.Sources[0].Uploaded = true
	a, err := s.RequestApproval("alice", team.ID, j, 0)
	if err != nil {
		t.Fatal(err)
	}
	j.TemplateText = "different"
	if _, err = s.DecideApproval("admin", team.ID, j, a.Revision, "approved", ""); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

func TestApprovalReceiptCannotCrossRequestRevision(t *testing.T) {
	s, _, _ := testStore(t)
	s.CreateUser("admin", "alice", "alice-password-123")
	team, _ := s.CreateTeam("admin", "Review")
	s.SetMember("admin", team.ID, "alice", RoleOperator, team.Revision)
	j := approvalJob()
	a, err := s.RequestApproval("alice", team.ID, j, 0)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.ReviewReceipt("admin", team.ID, j)
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.RequestApproval("alice", team.ID, j, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideApproval("admin", team.ID, j, a.Revision, "approved", "", receipt); !errors.Is(err, ErrConflict) {
		t.Fatal("old revision receipt accepted", err)
	}
	receipt, err = s.ReviewReceipt("admin", team.ID, j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideApproval("admin", team.ID, j, a.Revision, "approved", "", receipt); err != nil {
		t.Fatal(err)
	}
}
