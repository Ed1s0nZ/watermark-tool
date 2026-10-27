package workspace

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func samplePolicy(t *testing.T) PolicyContent {
	t.Helper()
	var p PolicyContent
	err := json.Unmarshal([]byte(`{"mode":"dual","template":"{name} · {recipient_id}","style":{"mode":"single","position":"bottomRight","size":28,"opacity":0.35,"color":"#365bdb","angle":-30},"quality":{},"output":{"format":"png"}}`), &p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestSharedPolicyVersionsPublishRollbackAndIsolation(t *testing.T) {
	s, dir, key := testStore(t)
	s.CreateUser("admin", "alice", "alice-password-123")
	s.CreateUser("admin", "bob", "bob-password-123")
	team, _ := s.CreateTeam("admin", "Team")
	other, _ := s.CreateTeam("admin", "Other")
	team, _ = s.SetMember("admin", team.ID, "alice", RoleOperator, team.Revision)
	s.SetMember("admin", team.ID, "bob", RoleReviewer, team.Revision)
	p := samplePolicy(t)
	if _, err := s.SavePolicy("alice", team.ID, "", "Denied", 0, p); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	saved, err := s.SavePolicy("admin", team.ID, "", "Internal", 0, p)
	if err != nil {
		t.Fatal(err)
	}
	p.Style["color"] = "#ffffff"
	hidden, _ := s.SharedPolicies("alice", team.ID)
	if len(hidden) != 0 {
		t.Fatal("draft exposed", hidden)
	}
	saved, err = s.PublishPolicy("admin", team.ID, saved.ID, saved.Revision, 1)
	if err != nil {
		t.Fatal(err)
	}
	version1 := saved.Versions[0]
	if version1.Content.Style["color"] != "#365bdb" {
		t.Fatal("caller alias")
	}
	p.Template = "second {name}"
	saved, err = s.SavePolicy("admin", team.ID, saved.ID, "Internal v2", saved.Revision, p)
	if err != nil {
		t.Fatal(err)
	}
	members, err := s.SharedPolicies("bob", team.ID)
	if err != nil || len(members) != 1 || len(members[0].Versions) != 1 || members[0].Versions[0].Number != 1 {
		t.Fatal(members, err)
	}
	if members[0].Versions[0].SHA256 != version1.SHA256 {
		t.Fatal("published draft changed")
	}
	if _, err = s.PublishPolicy("alice", team.ID, saved.ID, saved.Revision, 2); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	if _, err = s.PublishPolicy("admin", other.ID, saved.ID, saved.Revision, 2); !errors.Is(err, ErrMissing) {
		t.Fatal(err)
	}
	if _, err = s.PublishPolicy("admin", team.ID, saved.ID, saved.Revision-1, 2); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	saved, err = s.PublishPolicy("admin", team.ID, saved.ID, saved.Revision, 2)
	if err != nil {
		t.Fatal(err)
	}
	saved, err = s.PublishPolicy("admin", team.ID, saved.ID, saved.Revision, 1)
	if err != nil || saved.Versions[0].SHA256 != version1.SHA256 || len(saved.Versions) != 2 {
		t.Fatal(saved, err)
	}
	saved.Versions[0].Content.Style["color"] = "mutated"
	members, _ = s.SharedPolicies("bob", team.ID)
	if members[0].Versions[0].Content.Style["color"] != "#365bdb" {
		t.Fatal("read alias")
	}
	s.Close()
	s, err = Open(dir, key, "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	members, err = s.SharedPolicies("admin", team.ID)
	if err != nil || len(members) != 1 || members[0].Published != 1 || len(members[0].Versions) != 2 {
		t.Fatal(members, err)
	}
}
func TestSharedPolicyRejectsPrivateArtworkAndInvalidValues(t *testing.T) {
	s, _, _ := testStore(t)
	for _, mutate := range []func(*PolicyContent){
		func(p *PolicyContent) { p.Template = "{unknown}" }, func(p *PolicyContent) { p.Template = strings.Repeat("a", 81) }, func(p *PolicyContent) { p.Style["mode"] = "unknown" },
		func(p *PolicyContent) { p.Output.JPEGQuality = 90 }, func(p *PolicyContent) {
			p.Layout = &PolicyLayout{Layers: "both", Logo: PolicyLogo{Position: "center", WidthRatio: 0.9, Opacity: 0.3}}
		},
	} {
		p := samplePolicy(t)
		mutate(&p)
		if _, err := s.SavePolicy("admin", PersonalTeam, "", "Invalid", 0, p); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	var p PolicyContent
	if decode([]byte(`{"mode":"dual","layout":{"layers":"logo","logo":{"sha256":"private-file","position":"center","width_ratio":0.1,"opacity":0.3}}}`), &p) == nil {
		t.Fatal("accepted private artwork identity")
	}
	content := samplePolicy(t)
	saved, err := s.SavePolicy("admin", PersonalTeam, "", "Bounded", 0, content)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < MaxPolicyVersions; i++ {
		saved, err = s.SavePolicy("admin", PersonalTeam, saved.ID, saved.Name, saved.Revision, content)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.SavePolicy("admin", PersonalTeam, saved.ID, saved.Name, saved.Revision, content); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if _, err = s.PublishPolicy("admin", PersonalTeam, saved.ID, saved.Revision, 1); err != nil {
		t.Fatal("version limit blocked rollback", err)
	}
}
