package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func testStore(t *testing.T) (*Store, string, []byte) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private")
	key := bytes.Repeat([]byte{31}, 32)
	s, err := Open(dir, key, "admin")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir, key
}
func TestEncryptedPersistenceAndFailClosed(t *testing.T) {
	s, dir, key := testStore(t)
	user, err := s.CreateUser("admin", "alice", "test-password-alice")
	if err != nil {
		t.Fatal(err)
	}
	team, err := s.CreateTeam("admin", "Private client X")
	if err != nil {
		t.Fatal(err)
	}
	team, err = s.SetMember("admin", team.ID, "alice", RoleOperator, team.Revision)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "workspace-v1.enc"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"alice", "test-password-alice", "Private client X", "password_hash"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("plaintext leak: %s", secret)
		}
	}
	if other, e := Open(dir, key, "admin"); e == nil {
		other.Close()
		t.Fatal("second writer accepted")
	}
	s.Close()
	if other, e := Open(dir, bytes.Repeat([]byte{32}, 32), "admin"); e == nil {
		other.Close()
		t.Fatal("wrong key accepted")
	}
	if other, e := Open(dir, key, "new-admin"); e == nil {
		other.Close()
		t.Fatal("silent administrator reassignment")
	}
	s, err = Open(dir, key, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if version, ok := s.Authenticate("alice", "test-password-alice"); !ok || version != user.Revision {
		t.Fatal("credential lost")
	}
	if loaded, e := s.Team("alice", team.ID); e != nil || loaded.Revision != team.Revision {
		t.Fatal("team lost", e)
	}
	events, e := s.Events("alice", team.ID, 0, 100)
	if e != nil || len(events) != 2 || events[0].Action != "team.create" {
		t.Fatal(events, e)
	}
	s.Close()
	raw[len(raw)-1] ^= 1
	os.WriteFile(filepath.Join(dir, "workspace-v1.enc"), raw, 0600)
	if other, e := Open(dir, key, "admin"); e == nil {
		other.Close()
		t.Fatal("tampering accepted")
	}
}
func TestAuthorizationRevisionAndRevocation(t *testing.T) {
	s, _, _ := testStore(t)
	alice, err := s.CreateUser("admin", "alice", "test-password-alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateUser("alice", "bob", "test-password-bob"); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	team, _ := s.CreateTeam("admin", "A")
	other, _ := s.CreateTeam("admin", "B")
	team, err = s.SetMember("admin", team.ID, "alice", RoleReviewer, team.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if s.Authorize("alice", team.ID, "read") != nil || s.Authorize("alice", team.ID, "approve") != nil {
		t.Fatal("review blocked")
	}
	if !errors.Is(s.Authorize("alice", team.ID, "produce"), ErrDenied) || !errors.Is(s.Authorize("alice", other.ID, "read"), ErrDenied) {
		t.Fatal("permission bypass")
	}
	if _, err = s.Team("alice", PersonalTeam); !errors.Is(err, ErrDenied) {
		t.Fatal("legacy records exposed")
	}
	if _, err = s.SetMember("alice", team.ID, "alice", RoleOwner, team.Revision); !errors.Is(err, ErrDenied) {
		t.Fatal("self escalation")
	}
	if _, err = s.SetMember("admin", team.ID, "alice", RoleOperator, team.Revision-1); !errors.Is(err, ErrConflict) {
		t.Fatal("stale edit accepted")
	}
	public, _ := s.Team("admin", team.ID)
	public.Members["alice"] = RoleOwner
	if !errors.Is(s.Authorize("alice", team.ID, "produce"), ErrDenied) {
		t.Fatal("mutable public alias")
	}
	changed, err := s.UpdateUser("admin", "alice", alice.Revision, true, "new-password-alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Authenticate("alice", "test-password-alice"); ok {
		t.Fatal("old password accepted")
	}
	if v, ok := s.Authenticate("alice", "new-password-alice"); !ok || v != changed.Revision {
		t.Fatal("new credential not versioned")
	}
	changed, err = s.UpdateUser("admin", "alice", changed.Revision, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.CredentialVersion("alice"); ok {
		t.Fatal("disabled credential active")
	}
	if !errors.Is(s.Authorize("alice", team.ID, "read"), ErrDenied) {
		t.Fatal("disabled member active")
	}
	if _, err = s.SetMember("admin", team.ID, "admin", "", team.Revision); !errors.Is(err, ErrDenied) {
		t.Fatal("recovery owner removed")
	}
}
func TestConcurrentUpdatesAndWriteFailure(t *testing.T) {
	s, dir, key := testStore(t)
	s.CreateUser("admin", "alice", "test-password-alice")
	team, _ := s.CreateTeam("admin", "A")
	var wg sync.WaitGroup
	var success atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.SetMember("admin", team.ID, "alice", RoleOperator, team.Revision)
			if err == nil {
				success.Add(1)
			} else if !errors.Is(err, ErrConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatal("lost update", success.Load())
	}
	s.write = func(string, []byte) error { return errors.New("disk full") }
	if _, err := s.CreateTeam("admin", "must not persist"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := s.Teams("admin"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("poisoned state served")
	}
	s.Close()
	reopened, err := Open(dir, key, "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	teams, _ := reopened.Teams("admin")
	if len(teams) != 2 {
		t.Fatal("failed state committed", teams)
	}
}
func TestPrivateDirectoryAndMissingCipher(t *testing.T) {
	key := bytes.Repeat([]byte{31}, 32)
	dir := t.TempDir()
	os.Chmod(dir, 0755)
	if s, err := Open(dir, key, "admin"); err == nil {
		s.Close()
		t.Fatal("public directory accepted")
	}
	os.Chmod(dir, 0700)
	os.WriteFile(filepath.Join(dir, "unexpected"), []byte("data"), 0600)
	if s, err := Open(dir, key, "admin"); err == nil {
		s.Close()
		t.Fatal("silently bootstrapped over missing state")
	}
}

func TestCapacityReserveAllowsAdministrativeRevocation(t *testing.T) {
	s, _, _ := testStore(t)
	user, err := s.CreateUser("admin", "alice", "test-password-alice")
	if err != nil {
		t.Fatal(err)
	}
	s.data.Events = make([]Event, maxAudit-512)
	for i := range s.data.Events {
		s.data.Events[i] = Event{Sequence: uint64(i + 1), At: now(), Actor: "admin", Action: "test", Target: ""}
	}
	if _, err := s.CreateTeam("admin", "must fail"); !errors.Is(err, ErrLimit) {
		t.Fatal("ordinary writes consumed security reserve", err)
	}
	if _, err := s.UpdateUser("admin", "alice", user.Revision, false, ""); err != nil {
		t.Fatal("capacity prevented user revocation", err)
	}
	if _, ok := s.CredentialVersion("alice"); ok {
		t.Fatal("user remains active")
	}
}
