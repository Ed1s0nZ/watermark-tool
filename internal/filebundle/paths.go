package filebundle

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SafePath retains the original Unicode spelling but rejects nonportable or ambiguous archive paths.
func SafePath(name string) (string, error) {
	if name == "" || len(name) > 512 || !utf8.ValidString(name) || strings.ContainsAny(name, "\\:<>\"|?*") || strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return "", ErrPath
	}
	parts := strings.Split(name, "/")
	if len(parts) > 16 {
		return "", ErrPath
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", ErrPath
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return "", ErrPath
			}
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return "", ErrPath
		}
	}
	return name, nil
}
func pathKey(name string) string { return cases.Fold().String(norm.NFC.String(name)) }

type pathSet struct {
	entries map[string]bool
	parents map[string]bool
}

func newPathSet() *pathSet { return &pathSet{entries: map[string]bool{}, parents: map[string]bool{}} }
func (s *pathSet) add(name string, directory bool) error {
	key := pathKey(name)
	if _, exists := s.entries[key]; exists {
		return ErrPath
	}
	if !directory && s.parents[key] {
		return ErrPath
	}
	parts := strings.Split(key, "/")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], "/")
		if dir, exists := s.entries[parent]; exists && !dir {
			return ErrPath
		}
		s.parents[parent] = true
	}
	s.entries[key] = directory
	return nil
}
