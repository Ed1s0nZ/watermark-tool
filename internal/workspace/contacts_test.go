package workspace

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"watermark-tool/internal/distribution"
)

func TestContactsScopeSnapshotAndRevision(t *testing.T) {
	s, dir, key := testStore(t)
	s.CreateUser("admin", "alice", "alice-password-123")
	s.CreateUser("admin", "bob", "bob-password-123")
	team, _ := s.CreateTeam("admin", "Clients")
	other, _ := s.CreateTeam("admin", "Other")
	team, _ = s.SetMember("admin", team.ID, "alice", RoleOperator, team.Revision)
	s.SetMember("admin", team.ID, "bob", RoleReviewer, team.Revision)
	first, err := s.PutContact("alice", team.ID, "", 0, ContactValue{RecipientID: "001", Name: "张,三", Groups: []string{"Sales", "客户", "Sales"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.PutContact("alice", team.ID, "", 0, ContactValue{RecipientID: "002", Name: "Alice", Groups: []string{"Sales"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PutContact("bob", team.ID, "", 0, ContactValue{RecipientID: "003", Name: "Denied"}); !errors.Is(err, ErrDenied) {
		t.Fatal("reviewer wrote", err)
	}
	if _, err = s.SearchContacts("alice", other.ID, "", "", 1, 20); !errors.Is(err, ErrDenied) {
		t.Fatal("cross-team search", err)
	}
	found, err := s.SearchContacts("bob", team.ID, "", "Sales", 1, 1)
	if err != nil || found.Total != 2 || found.TeamTotal != 2 || len(found.Items) != 1 || len(found.Groups) != 2 {
		t.Fatal(found, err)
	}
	found.Items[0].Groups[0] = "mutated"
	snapshot, err := s.SelectContacts("bob", team.ID, []string{first.ID, second.ID})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := distribution.ParseRecipients(snapshot.CSV)
	if err != nil || len(rows) != 2 || rows[0].Name != "张,三" || len(snapshot.SHA256) != 64 {
		t.Fatal(rows, snapshot, err)
	}
	first, err = s.PutContact("alice", team.ID, first.ID, first.Revision, ContactValue{RecipientID: "001", Name: "Renamed", Groups: []string{"Sales"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snapshot.CSV, "张,三") || snapshot.Items[0].Name != "张,三" {
		t.Fatal("history snapshot changed")
	}
	if _, err = s.PutContact("alice", team.ID, first.ID, first.Revision-1, ContactValue{RecipientID: "001", Name: "stale"}); !errors.Is(err, ErrConflict) {
		t.Fatal("stale overwrite", err)
	}
	if _, err = s.PutContact("alice", team.ID, "", 0, ContactValue{RecipientID: "001", Name: "duplicate"}); !errors.Is(err, ErrDuplicate) {
		t.Fatal(err)
	}
	if err = s.DeleteContact("alice", team.ID, second.ID, second.Revision+1); !errors.Is(err, ErrConflict) {
		t.Fatal("stale deletion", err)
	}
	if err = s.DeleteContact("alice", team.ID, second.ID, second.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SelectContacts("alice", team.ID, []string{second.ID}); !errors.Is(err, ErrMissing) {
		t.Fatal(err)
	}
	s.Close()
	reopened, err := Open(dir, key, "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := reopened.SearchContacts("alice", team.ID, "renam", "", 1, 20)
	if err != nil || len(persisted.Items) != 1 || persisted.Items[0].Revision != first.Revision {
		t.Fatal(persisted, err)
	}
}
func TestContactImportAtomicLimitsAndCSV(t *testing.T) {
	s, _, _ := testStore(t)
	imported, err := s.ImportContacts("admin", PersonalTeam, "recipient_id,name,groups\n01,Alice,Sales|客户\n02,\"Bob, B\",Sales")
	if err != nil || len(imported) != 2 {
		t.Fatal(imported, err)
	}
	for _, text := range []string{"recipient_id,name\n03,New\n01,Conflict", "recipient_id,name\n03,New\n03,Duplicate"} {
		result, err := s.ImportContacts("admin", PersonalTeam, text)
		var failure *ContactImportError
		if !errors.Is(err, ErrDuplicate) || !errors.As(err, &failure) || failure.Row != 3 || len(result) != 0 {
			t.Fatal(result, err)
		}
		found, _ := s.SearchContacts("admin", PersonalTeam, "", "", 1, 100)
		if found.Total != 2 {
			t.Fatal("partial import", found.Total)
		}
	}
	if _, err = s.SelectContacts("admin", PersonalTeam, make([]string, 21)); !errors.Is(err, ErrLimit) {
		t.Fatal("recipient cap bypass", err)
	}
	if _, err = ParseContactCSV(strings.Repeat("a", MaxContactImportBytes+1)); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if _, err = ParseContactCSV("recipient_id,name\n01,\xff"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	malicious := Contact{RecipientID: "=1+1", Name: "\u200b=SUM(1)", Groups: []string{"Sales"}}
	raw := ExportContactCSV([]Contact{malicious})
	rows, err := csv.NewReader(strings.NewReader(raw)).ReadAll()
	if err != nil || !strings.HasPrefix(rows[1][0], "'") || !strings.HasPrefix(rows[1][1], "'") {
		t.Fatal(raw, err)
	}
	lossless, _ := json.Marshal(malicious)
	if !strings.Contains(string(lossless), "=1+1") {
		t.Fatal("JSON export changed data")
	}
}
