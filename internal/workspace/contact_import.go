package workspace

import (
	"encoding/csv"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ContactImportError struct {
	Cause error
	Row   int
}

func (e *ContactImportError) Error() string { return e.Cause.Error() }
func (e *ContactImportError) Unwrap() error { return e.Cause }
func ParseContactCSV(text string) ([]ContactValue, error) {
	if len(text) > MaxContactImportBytes {
		return nil, ErrLimit
	}
	if !utf8.ValidString(text) {
		return nil, ErrInvalid
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\ufeff")))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil || len(header) < 2 || len(header) > 3 || header[0] != "recipient_id" || header[1] != "name" || len(header) == 3 && header[2] != "groups" {
		return nil, ErrInvalid
	}
	reader.FieldsPerRecord = len(header)
	rows := []ContactValue{}
	seen := map[string]bool{}
	for row := 2; ; row++ {
		cells, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &ContactImportError{ErrInvalid, row}
		}
		if len(rows) >= MaxContacts {
			return nil, ErrLimit
		}
		v := ContactValue{RecipientID: strings.TrimSpace(cells[0]), Name: strings.TrimSpace(cells[1]), Groups: []string{}}
		if len(cells) == 3 && strings.TrimSpace(cells[2]) != "" {
			for _, g := range strings.Split(cells[2], "|") {
				v.Groups = append(v.Groups, strings.TrimSpace(g))
			}
		}
		v, err = normalizeContact(v)
		if err != nil {
			return nil, &ContactImportError{err, row}
		}
		if seen[v.RecipientID] {
			return nil, &ContactImportError{ErrDuplicate, row}
		}
		seen[v.RecipientID] = true
		rows = append(rows, v)
	}
	if len(rows) == 0 {
		return nil, ErrInvalid
	}
	return rows, nil
}
func (s *Store) ImportContacts(actor, team, text string) ([]Contact, error) {
	values, err := ParseContactCSV(text)
	if err != nil {
		return nil, err
	}
	result := []Contact{}
	err = s.change(actor, team, "recipient.import", team, func(d *state) error {
		if !s.can(*d, actor, team, "produce") {
			return ErrDenied
		}
		if d.Contacts == nil {
			d.Contacts = map[string]Contact{}
		}
		existing := map[string]bool{}
		for _, contact := range d.Contacts {
			if contact.TeamID == team {
				existing[contact.RecipientID] = true
			}
		}
		if len(existing)+len(values) > MaxContacts {
			return ErrLimit
		}
		for index, v := range values {
			if existing[v.RecipientID] {
				return &ContactImportError{ErrDuplicate, index + 2}
			}
			id, err := newID()
			if err != nil {
				return err
			}
			c := Contact{ID: id, TeamID: team, RecipientID: v.RecipientID, Name: v.Name, Groups: v.Groups, Revision: 1, UpdatedAt: now()}
			d.Contacts[id] = c
			result = append(result, copyContact(c))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ExportContactCSV is spreadsheet-safe. JSON remains the lossless export format.
func ExportContactCSV(contacts []Contact) string {
	safe := func(v string) string {
		trimmed := strings.TrimLeftFunc(v, func(r rune) bool { return unicode.IsSpace(r) || unicode.In(r, unicode.Cf) })
		if strings.ContainsAny(v, "\r\n") || len(trimmed) > 0 && strings.ContainsRune("=+@-", rune(trimmed[0])) {
			return "'" + v
		}
		return v
	}
	var text strings.Builder
	writer := csv.NewWriter(&text)
	writer.Write([]string{"recipient_id", "name", "groups"})
	for _, c := range contacts {
		writer.Write([]string{safe(c.RecipientID), safe(c.Name), safe(strings.Join(c.Groups, "|"))})
	}
	writer.Flush()
	return text.String()
}
