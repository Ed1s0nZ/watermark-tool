package pdfmark

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var pageNumberPattern = regexp.MustCompile(`^[1-9][0-9]{0,2}$`)

func SelectedPages(selection string, count int, skipCover bool) ([]int, error) {
	if count < 1 || count > MaxPages || len(selection) > 4096 {
		return nil, ErrSelection
	}
	seen := map[int]bool{}
	selection = strings.TrimSpace(selection)
	if selection == "" || selection == "all" {
		for p := 1; p <= count; p++ {
			seen[p] = true
		}
	} else {
		for _, part := range strings.Split(selection, ",") {
			bounds := strings.Split(strings.TrimSpace(part), "-")
			if len(bounds) > 2 || len(bounds) == 0 {
				return nil, ErrSelection
			}
			parse := func(raw string) (int, error) {
				raw = strings.TrimSpace(raw)
				if !pageNumberPattern.MatchString(raw) {
					return 0, ErrSelection
				}
				n, _ := strconv.Atoi(raw)
				if n > count {
					return 0, ErrSelection
				}
				return n, nil
			}
			start, err := parse(bounds[0])
			if err != nil {
				return nil, err
			}
			end := start
			if len(bounds) == 2 {
				end, err = parse(bounds[1])
				if err != nil || end < start {
					return nil, ErrSelection
				}
			}
			for p := start; p <= end; p++ {
				seen[p] = true
			}
		}
	}
	if skipCover {
		delete(seen, 1)
	}
	result := []int{}
	for p := range seen {
		result = append(result, p)
	}
	sort.Ints(result)
	if len(result) == 0 {
		return nil, ErrSelection
	}
	return result, nil
}
func validText(s string) bool {
	if !utf8.ValidString(s) || len(s) > 500 || strings.TrimSpace(s) == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}
func MakePlan(count int, p Params) ([]PagePlan, error) {
	if !idPattern.MatchString(p.DocumentID) || !keyPattern.MatchString(p.KeyID) || p.CreatedAt.IsZero() || !validText(p.Text) || len(p.Overrides) > MaxPages {
		return nil, ErrText
	}
	selected, err := SelectedPages(p.Pages, count, p.SkipCover)
	if err != nil {
		return nil, err
	}
	set := map[int]bool{}
	for _, n := range selected {
		set[n] = true
	}
	for n, text := range p.Overrides {
		if !set[n] || !validText(text) {
			return nil, ErrText
		}
	}
	plan := []PagePlan{}
	for _, n := range selected {
		text := p.Text
		if override, ok := p.Overrides[n]; ok {
			text = override
		}
		probe := strings.NewReplacer("{page}", "", "{pages}", "").Replace(text)
		if strings.ContainsAny(probe, "{}") {
			return nil, ErrText
		}
		text = strings.NewReplacer("{page}", strconv.Itoa(n), "{pages}", strconv.Itoa(count)).Replace(text)
		if !validText(text) {
			return nil, ErrText
		}
		plan = append(plan, PagePlan{Page: n, Text: text})
	}
	return plan, nil
}
