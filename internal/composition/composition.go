// Package composition validates declarative visible artwork identities; it does not recognize logos.
package composition

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
)

var ErrArtwork = errors.New("artwork_invalid")
var hash = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type Logo struct {
	SHA256     string  `json:"sha256"`
	Position   string  `json:"position"`
	WidthRatio float64 `json:"width_ratio"`
	Opacity    float64 `json:"opacity"`
}
type Artwork struct {
	Layers     string            `json:"layers"`
	Logo       *Logo             `json:"logo,omitempty"`
	Exclusions map[string][]Rect `json:"exclusions,omitempty"`
}

func number(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func (r Rect) Valid() bool {
	return number(r.X) && number(r.Y) && number(r.Width) && number(r.Height) && r.X >= 0 && r.Y >= 0 && r.Width > 0 && r.Height > 0 && r.X+r.Width <= 1.000000001 && r.Y+r.Height <= 1.000000001
}
func Position(p string) bool {
	switch p {
	case "topLeft", "topRight", "center", "bottomLeft", "bottomRight":
		return true
	}
	return false
}
func Text(a *Artwork) bool { return a == nil || a.Layers != "logo" }
func Validate(a *Artwork, sources map[string]bool) bool {
	if a == nil {
		return true
	}
	if a.Layers != "text" && a.Layers != "logo" && a.Layers != "both" {
		return false
	}
	if a.Layers == "text" {
		if a.Logo != nil {
			return false
		}
	} else {
		l := a.Logo
		if l == nil || !hash.MatchString(l.SHA256) || !Position(l.Position) || !number(l.WidthRatio) || l.WidthRatio < 0.05 || l.WidthRatio > 0.4 || !number(l.Opacity) || l.Opacity < 0.05 || l.Opacity > 1 {
			return false
		}
	}
	if len(a.Exclusions) > 5 {
		return false
	}
	for sha, rs := range a.Exclusions {
		if !hash.MatchString(sha) || sources != nil && !sources[sha] || len(rs) > 8 {
			return false
		}
		for _, r := range rs {
			if !r.Valid() {
				return false
			}
		}
	}
	return true
}
func Clone(a *Artwork) *Artwork {
	if a == nil {
		return nil
	}
	b, _ := json.Marshal(a)
	var out Artwork
	_ = json.Unmarshal(b, &out)
	return &out
}
func Normalize(a *Artwork) *Artwork {
	a = Clone(a)
	if a == nil {
		return nil
	}
	for k, rs := range a.Exclusions {
		if len(rs) == 0 {
			delete(a.Exclusions, k)
		}
	}
	if a.Layers == "text" && a.Logo == nil && len(a.Exclusions) == 0 {
		return nil
	}
	return a
}
