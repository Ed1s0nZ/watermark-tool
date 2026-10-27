package workspace

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxContacts = 1000
const MaxContactGroups = 100
const MaxContactImportBytes = 1 << 20

type Contact struct {
	ID          string    `json:"id"`
	TeamID      string    `json:"team_id"`
	RecipientID string    `json:"recipient_id"`
	Name        string    `json:"name"`
	Groups      []string  `json:"groups"`
	Revision    uint64    `json:"revision"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type ContactValue struct {
	RecipientID string   `json:"recipient_id"`
	Name        string   `json:"name"`
	Groups      []string `json:"groups"`
}
type ContactSearch struct {
	Items     []Contact `json:"items"`
	Total     int       `json:"total"`
	Groups    []string  `json:"groups"`
	TeamTotal int       `json:"team_total"`
}
type ContactSelection struct {
	Items  []Contact `json:"items"`
	CSV    string    `json:"csv"`
	SHA256 string    `json:"sha256"`
}

func copyContact(c Contact) Contact { c.Groups = append([]string{}, c.Groups...); return c }
func normalizeContact(v ContactValue) (ContactValue, error) {
	if !validName(v.RecipientID, 64) || !validName(v.Name, 128) || len(v.Groups) > 10 {
		return v, ErrInvalid
	}
	groups := map[string]bool{}
	for _, g := range v.Groups {
		if !validName(g, 32) || strings.Contains(g, "|") {
			return v, ErrInvalid
		}
		groups[g] = true
	}
	v.Groups = []string{}
	for g := range groups {
		v.Groups = append(v.Groups, g)
	}
	sort.Strings(v.Groups)
	return v, nil
}
func (s *Store) validateContacts(d state) error {
	counts := map[string]int{}
	businessIDs := map[string]bool{}
	groups := map[string]map[string]bool{}
	for id, c := range d.Contacts {
		if !teamPattern.MatchString(id) || id != c.ID || c.Revision == 0 || c.UpdatedAt.IsZero() {
			return ErrInvalid
		}
		if _, ok := d.Teams[c.TeamID]; !ok {
			return ErrInvalid
		}
		normalized, err := normalizeContact(ContactValue{c.RecipientID, c.Name, c.Groups})
		if err != nil {
			return err
		}
		if strings.Join(normalized.Groups, "\x00") != strings.Join(c.Groups, "\x00") {
			return ErrInvalid
		}
		counts[c.TeamID]++
		if counts[c.TeamID] > MaxContacts {
			return ErrLimit
		}
		key := c.TeamID + "\x00" + c.RecipientID
		if businessIDs[key] {
			return ErrDuplicate
		}
		businessIDs[key] = true
		if groups[c.TeamID] == nil {
			groups[c.TeamID] = map[string]bool{}
		}
		for _, g := range c.Groups {
			groups[c.TeamID][g] = true
		}
		if len(groups[c.TeamID]) > MaxContactGroups {
			return ErrLimit
		}
	}
	return nil
}
func (s *Store) PutContact(actor, team, id string, revision uint64, value ContactValue) (Contact, error) {
	value, err := normalizeContact(value)
	if err != nil {
		return Contact{}, err
	}
	create := id == ""
	if create {
		id, err = newID()
		if err != nil {
			return Contact{}, err
		}
	}
	var result Contact
	err = s.change(actor, team, "recipient.save", id, func(d *state) error {
		if !s.can(*d, actor, team, "produce") {
			return ErrDenied
		}
		if d.Contacts == nil {
			d.Contacts = map[string]Contact{}
		}
		if create {
			if revision != 0 {
				return ErrInvalid
			}
			revision = 1
		} else {
			current, ok := d.Contacts[id]
			if !ok || current.TeamID != team {
				return ErrMissing
			}
			if current.Revision != revision {
				return ErrConflict
			}
			revision++
		}
		for otherID, c := range d.Contacts {
			if otherID != id && c.TeamID == team && c.RecipientID == value.RecipientID {
				return ErrDuplicate
			}
		}
		result = Contact{ID: id, TeamID: team, RecipientID: value.RecipientID, Name: value.Name, Groups: value.Groups, Revision: revision, UpdatedAt: now()}
		d.Contacts[id] = result
		return nil
	})
	return copyContact(result), err
}
func (s *Store) DeleteContact(actor, team, id string, revision uint64) error {
	return s.change(actor, team, "recipient.delete", id, func(d *state) error {
		if !s.can(*d, actor, team, "produce") {
			return ErrDenied
		}
		c, ok := d.Contacts[id]
		if !ok || c.TeamID != team {
			return ErrMissing
		}
		if c.Revision != revision {
			return ErrConflict
		}
		delete(d.Contacts, id)
		return nil
	})
}
func (s *Store) SearchContacts(actor, team, q, group string, page, size int) (ContactSearch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := ContactSearch{Items: []Contact{}, Groups: []string{}}
	if s.closed || s.poisoned {
		return result, ErrUnavailable
	}
	if !s.can(s.data, actor, team, "read") {
		return result, ErrDenied
	}
	if len(q) > 128 || !utf8.ValidString(q) || len(group) > 32 || !utf8.ValidString(group) || page < 1 || page > 1000000 || size < 1 || size > 100 {
		return result, ErrInvalid
	}
	groups := map[string]bool{}
	q = strings.ToLower(q)
	for _, c := range s.data.Contacts {
		if c.TeamID != team {
			continue
		}
		result.TeamTotal++
		hasGroup := group == ""
		for _, g := range c.Groups {
			groups[g] = true
			if g == group {
				hasGroup = true
			}
		}
		if hasGroup && (q == "" || strings.Contains(strings.ToLower(c.RecipientID+"\n"+c.Name), q)) {
			result.Items = append(result.Items, copyContact(c))
		}
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].RecipientID < result.Items[j].RecipientID })
	for g := range groups {
		result.Groups = append(result.Groups, g)
	}
	sort.Strings(result.Groups)
	result.Total = len(result.Items)
	start := (page - 1) * size
	if start >= len(result.Items) {
		result.Items = []Contact{}
	} else {
		end := start + size
		if end > len(result.Items) {
			end = len(result.Items)
		}
		result.Items = result.Items[start:end]
	}
	return result, nil
}
func (s *Store) SelectContacts(actor, team string, ids []string) (ContactSelection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := ContactSelection{Items: []Contact{}}
	if s.closed || s.poisoned {
		return result, ErrUnavailable
	}
	if !s.can(s.data, actor, team, "read") {
		return result, ErrDenied
	}
	if len(ids) < 1 || len(ids) > 20 {
		return result, ErrLimit
	}
	seen := map[string]bool{}
	var text strings.Builder
	writer := csv.NewWriter(&text)
	writer.Write([]string{"recipient_id", "name"})
	for _, id := range ids {
		if seen[id] {
			return result, ErrInvalid
		}
		seen[id] = true
		contact, ok := s.data.Contacts[id]
		if !ok || contact.TeamID != team {
			return result, ErrMissing
		}
		result.Items = append(result.Items, copyContact(contact))
		writer.Write([]string{contact.RecipientID, contact.Name})
	}
	writer.Flush()
	if writer.Error() != nil {
		return result, ErrUnavailable
	}
	result.CSV = text.String()
	raw, _ := json.Marshal(result.Items)
	digest := sha256.Sum256(raw)
	result.SHA256 = hex.EncodeToString(digest[:])
	return result, nil
}

func (s *Store) ExportContacts(actor, team string) ([]Contact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return nil, ErrUnavailable
	}
	if !s.can(s.data, actor, team, "read") {
		return nil, ErrDenied
	}
	result := []Contact{}
	for _, c := range s.data.Contacts {
		if c.TeamID == team {
			result = append(result, copyContact(c))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].RecipientID < result[j].RecipientID })
	return result, nil
}
