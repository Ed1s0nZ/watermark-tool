package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
	"watermark-tool/internal/composition"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/output"
	"watermark-tool/internal/quality"
)

// Shared policies contain layout scalars only, never source files, logo identities or exclusions.
type PolicyLogo struct {
	Position   string  `json:"position"`
	WidthRatio float64 `json:"width_ratio"`
	Opacity    float64 `json:"opacity"`
}
type PolicyLayout struct {
	Layers string     `json:"layers"`
	Logo   PolicyLogo `json:"logo"`
}
type PolicyContent struct {
	Mode     string          `json:"mode"`
	Template string          `json:"template"`
	Style    map[string]any  `json:"style"`
	Quality  quality.Policy  `json:"quality"`
	Layout   *PolicyLayout   `json:"layout,omitempty"`
	Output   *output.Options `json:"output,omitempty"`
}

func validPolicyContent(p PolicyContent) bool {
	if (p.Mode != "blind" && p.Mode != "dual") || !distribution.ValidStyle(p.Style) || !p.Quality.Valid() || !utf8.ValidString(p.Template) || utf8.RuneCountInString(p.Template) > 80 {
		return false
	}
	probe := strings.NewReplacer("{name}", "", "{recipient_id}", "").Replace(p.Template)
	if strings.ContainsAny(probe, "{}") {
		return false
	}
	for _, r := range p.Template {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	if strings.TrimSpace(p.Template) == "" && (p.Layout == nil || p.Layout.Layers != "logo") {
		return false
	}
	if p.Output != nil {
		if p.Output.Format != "png" && p.Output.Format != "jpeg" {
			return false
		}
		if _, err := output.Normalize(p.Output); err != nil {
			return false
		}
	}
	if p.Layout != nil {
		l := p.Layout.Logo
		if (p.Layout.Layers != "text" && p.Layout.Layers != "logo" && p.Layout.Layers != "both") || !composition.Position(l.Position) || math.IsNaN(l.WidthRatio) || math.IsInf(l.WidthRatio, 0) || l.WidthRatio < 0.05 || l.WidthRatio > 0.4 || math.IsNaN(l.Opacity) || math.IsInf(l.Opacity, 0) || l.Opacity < 0.05 || l.Opacity > 1 {
			return false
		}
	}
	return true
}
func policyDigest(p PolicyContent) string {
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func clonePolicy(p SharedPolicy) SharedPolicy {
	raw, _ := json.Marshal(p)
	var result SharedPolicy
	_ = json.Unmarshal(raw, &result)
	return result
}
func clonePolicyContent(p PolicyContent) (PolicyContent, error) {
	raw, err := json.Marshal(p)
	if err != nil || len(raw) > 16<<10 {
		return PolicyContent{}, ErrInvalid
	}
	var result PolicyContent
	if decode(raw, &result) != nil || !validPolicyContent(result) {
		return PolicyContent{}, ErrInvalid
	}
	return result, nil
}
