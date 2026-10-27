package pdfmark

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSelectionAndExpandedTextBounds(t *testing.T) {
	selected, err := SelectedPages("3, 1-2,2", 3, true)
	if err != nil || !reflect.DeepEqual(selected, []int{2, 3}) {
		t.Fatal(selected, err)
	}
	for _, selection := range []string{"0", "4", "2-1", "1-", "-1", "1,,2", "1;2", "01", "1-2-3"} {
		if _, err = SelectedPages(selection, 3, false); !errors.Is(err, ErrSelection) {
			t.Fatal(selection, err)
		}
	}
	if _, err = SelectedPages("1", 3, true); !errors.Is(err, ErrSelection) {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Params){func(p *Params) { p.Text = "{unknown}" }, func(p *Params) { p.Text = strings.Repeat("a", 501) }, func(p *Params) { p.Overrides = map[int]string{1: "ignored cover override"} }, func(p *Params) { p.Text = "control\n" }} {
		p := params()
		mutate(&p)
		if _, err = MakePlan(3, p); !errors.Is(err, ErrText) {
			t.Fatal(err)
		}
	}
}
