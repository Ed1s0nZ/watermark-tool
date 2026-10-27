package workspace

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"watermark-tool/internal/distribution"
)

// Receipts prove that the review API served a server-generated thumbnail for this
// actor, approval revision and immutable snapshot. They do not prove human attention.
func (s *Store) reviewMAC(actor string, a Approval, expires string) string {
	h := hmac.New(sha256.New, DerivedKey(s.key, "review-receipt"))
	h.Write([]byte(strings.Join([]string{actor, a.TeamID, a.JobID, a.Snapshot, strconv.FormatUint(a.Revision, 10), expires}, "\x00")))
	return hex.EncodeToString(h.Sum(nil))
}
func (s *Store) ReviewReceipt(actor, team string, j distribution.Job) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return "", ErrUnavailable
	}
	if !s.can(s.data, actor, team, "approve") || actor == j.Operator {
		return "", ErrDenied
	}
	a, ok := s.data.Approvals[approvalKey(team, j.ID)]
	snapshot, err := JobApprovalDigest(team, j)
	if !ok || err != nil || a.Status != "pending" || a.Snapshot != snapshot {
		return "", ErrConflict
	}
	expires := strconv.FormatInt(now().Add(5*time.Minute).Unix(), 10)
	return expires + "." + s.reviewMAC(actor, a, expires), nil
}
func (s *Store) validReviewReceipt(actor string, a Approval, receipt string) bool {
	parts := strings.Split(receipt, ".")
	if len(parts) != 2 || len(parts[1]) != 64 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || expires <= now().Unix() || expires > now().Add(5*time.Minute).Unix() {
		return false
	}
	return hmac.Equal([]byte(parts[1]), []byte(s.reviewMAC(actor, a, parts[0])))
}
