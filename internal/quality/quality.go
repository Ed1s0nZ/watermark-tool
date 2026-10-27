// Package quality measures decoded RGB differences; it does not certify perception or robustness.
package quality

import (
	"errors"
	"image"
	"image/color"
	"math"
	"regexp"
	"watermark-tool/internal/blind"
)

var ErrPolicy = errors.New("quality_policy")
var ErrThreshold = errors.New("quality_failed")

type Policy struct {
	MinimumPSNR *float64 `json:"minimum_psnr_db,omitempty"`
}

func (p Policy) Valid() bool {
	return p.MinimumPSNR == nil || !math.IsNaN(*p.MinimumPSNR) && !math.IsInf(*p.MinimumPSNR, 0) && *p.MinimumPSNR >= 20 && *p.MinimumPSNR <= 60
}
func Clone(p *Policy) *Policy {
	if p == nil {
		return nil
	}
	out := *p
	if p.MinimumPSNR != nil {
		n := *p.MinimumPSNR
		out.MinimumPSNR = &n
	}
	return &out
}

type Metric struct {
	Status string   `json:"status"`
	DB     *float64 `json:"psnr_db,omitempty"`
}

func (m Metric) Valid() bool {
	return m.Status == "finite" && m.DB != nil && !math.IsNaN(*m.DB) && !math.IsInf(*m.DB, 0) && *m.DB >= 0 || (m.Status == "infinite" || m.Status == "unavailable") && m.DB == nil
}
func Measure(a, b image.Image) Metric {
	if a == nil || b == nil {
		return Metric{Status: "unavailable"}
	}
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() <= 0 || ab.Dy() <= 0 || ab.Size() != bb.Size() {
		return Metric{Status: "unavailable"}
	}
	var sum float64
	for y := 0; y < ab.Dy(); y++ {
		for x := 0; x < ab.Dx(); x++ {
			ac := color.NRGBAModel.Convert(a.At(ab.Min.X+x, ab.Min.Y+y)).(color.NRGBA)
			bc := color.NRGBAModel.Convert(b.At(bb.Min.X+x, bb.Min.Y+y)).(color.NRGBA)
			r, g, b := float64(ac.R)-float64(bc.R), float64(ac.G)-float64(bc.G), float64(ac.B)-float64(bc.B)
			sum += r*r + g*g + b*b
		}
	}
	if sum == 0 {
		return Metric{Status: "infinite"}
	}
	db := 10 * math.Log10(255*255/(sum/float64(ab.Dx()*ab.Dy()*3)))
	return Metric{Status: "finite", DB: &db}
}
func (p Policy) Accept(m Metric) bool {
	if !p.Valid() {
		return false
	}
	if p.MinimumPSNR == nil {
		return true
	}
	return m.Status == "infinite" || m.Status == "finite" && m.DB != nil && *m.DB >= *p.MinimumPSNR
}

type Summary struct {
	Encoding     *Metric `json:"encoding,omitempty"`
	Final        *Metric `json:"final,omitempty"`
	OutputSHA256 string  `json:"output_sha256,omitempty"`
	OutputBytes  int64   `json:"output_bytes,omitempty"`
	Format       string  `json:"format,omitempty"`
	JPEGQuality  int     `json:"jpeg_quality,omitempty"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Capacity     int     `json:"capacity_bytes"`
	Verified     bool    `json:"verified"`
	Embedding    Metric  `json:"embedding"`
	Visible      *Metric `json:"visible,omitempty"`
	Accepted     bool    `json:"accepted"`
}

func Compare(input, output, original image.Image, p Policy) Summary {
	b := input.Bounds()
	s := Summary{Width: b.Dx(), Height: b.Dy(), Capacity: blind.MaxTextBytes, Verified: true, Embedding: Measure(input, output)}
	if original != nil {
		m := Measure(original, input)
		s.Visible = &m
	}
	s.Accepted = p.Accept(s.Embedding)
	return s
}
func (s Summary) GateMetric() Metric {
	if s.Final != nil {
		return *s.Final
	}
	return s.Embedding
}
func (s Summary) Valid() bool {
	base := s.Width > 0 && s.Height > 0 && s.Width <= blind.MaxPixels/s.Height && s.Capacity == blind.MaxTextBytes && s.Embedding.Valid() && (s.Visible == nil || s.Visible.Valid())
	if s.Final == nil {
		return base && s.Verified && s.Encoding == nil && s.OutputSHA256 == "" && s.OutputBytes == 0 && s.Format == "" && s.JPEGQuality == 0
	}
	return base && s.Final.Valid() && s.Encoding != nil && s.Encoding.Valid() && regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(s.OutputSHA256) && s.OutputBytes > 0 && s.OutputBytes <= 50<<20 && (s.Format == "png" && s.JPEGQuality == 0 || s.Format == "jpeg" && s.JPEGQuality >= 60 && s.JPEGQuality <= 100) && (!s.Accepted || s.Verified)
}
func CloneSummary(s *Summary) *Summary {
	if s == nil {
		return nil
	}
	out := *s
	clone := func(m *Metric) *Metric {
		if m == nil {
			return nil
		}
		v := *m
		if v.DB != nil {
			db := *v.DB
			v.DB = &db
		}
		return &v
	}
	out.Embedding = *clone(&s.Embedding)
	out.Visible = clone(s.Visible)
	out.Encoding = clone(s.Encoding)
	out.Final = clone(s.Final)
	return &out
}
