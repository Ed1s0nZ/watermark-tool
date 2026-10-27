package composition

import (
	"math"
	"strings"
	"testing"
)

func TestArtworkBoundsAndClone(t *testing.T) {
	sha := strings.Repeat("a", 64)
	a := &Artwork{Layers: "both", Logo: &Logo{SHA256: sha, Position: "bottomRight", WidthRatio: 0.15, Opacity: 0.35}, Exclusions: map[string][]Rect{sha: {{X: 0, Y: 0, Width: 0.5, Height: 1}}}}
	if !Validate(a, map[string]bool{sha: true}) {
		t.Fatal(a)
	}
	b := Clone(a)
	b.Logo.Opacity = 1
	b.Exclusions[sha][0].X = 0.1
	if a.Logo.Opacity != 0.35 || a.Exclusions[sha][0].X != 0 {
		t.Fatal("alias")
	}
	for _, r := range []Rect{{Width: 0, Height: 1}, {X: -0.1, Width: 1, Height: 1}, {X: 0.5, Width: 0.6, Height: 1}, {X: math.NaN(), Width: 1, Height: 1}} {
		if r.Valid() {
			t.Fatal(r)
		}
	}
	if Validate(a, map[string]bool{strings.Repeat("b", 64): true}) {
		t.Fatal("unknown source")
	}
	a.Exclusions[sha] = make([]Rect, 9)
	if Validate(a, nil) {
		t.Fatal("region count")
	}
	if Normalize(&Artwork{Layers: "text"}) != nil {
		t.Fatal("default fingerprint")
	}
}
