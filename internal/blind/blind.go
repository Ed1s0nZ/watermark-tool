// Package blind implements the independent, versioned Go DCT image watermark.
// Its authentication protects the payload, not the surrounding image content.
package blind

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	Algorithm    = "go-dct-v1"
	MaxTextBytes = 34
	MaxPixels    = 8_000_000
	frameSize    = 64
	frameBits    = frameSize * 8
	minRepeat    = 3
	margin       = 64.0
)

var (
	ErrNotFound = errors.New("blind_not_found")
	ErrInvalid  = errors.New("blind_invalid")
	ErrCapacity = errors.New("blind_capacity")
	ErrImage    = errors.New("blind_image")
	ErrKey      = errors.New("blind_key")
	ErrText     = errors.New("blind_text")
)

type Result struct {
	Watermark string `json:"watermark"`
	Timestamp string `json:"timestamp"`
	Algorithm string `json:"algorithm"`
}

var basisA, basisB = basis(1, 2), basis(2, 1)

func basis(u, v int) (out [64]float64) {
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			out[y*8+x] = 0.25 * math.Cos(float64((2*x+1)*u)*math.Pi/16) * math.Cos(float64((2*y+1)*v)*math.Pi/16)
		}
	}
	return
}
func dimensions(img image.Image) (int, int, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || w > MaxPixels/h {
		return 0, 0, ErrImage
	}
	if (w/8)*(h/8) < frameBits*minRepeat {
		return 0, 0, ErrCapacity
	}
	if opaque, ok := img.(interface{ Opaque() bool }); ok && !opaque.Opaque() {
		return 0, 0, ErrImage
	}
	return w, h, nil
}

// permutation uses a domain-separated keyed stream, independent of math/rand versions.
func permutation(n int, key []byte) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	var counter uint64
	var block [8]byte
	for i := n - 1; i > 0; i-- {
		// Rejection sampling avoids modulo bias.
		size := uint64(i + 1)
		limit := uint64(math.MaxUint64) - (uint64(math.MaxUint64) % size)
		var value uint64
		for {
			binary.BigEndian.PutUint64(block[:], counter)
			counter++
			m := hmac.New(sha256.New, key)
			m.Write([]byte("go-dct-v1/blocks/"))
			m.Write(block[:])
			value = binary.BigEndian.Uint64(m.Sum(nil))
			if value < limit {
				break
			}
		}
		j := int(value % size)
		p[i], p[j] = p[j], p[i]
	}
	return p
}
func sign(data, key []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("go-dct-v1/payload/"))
	m.Write(data)
	return m.Sum(nil)[:16]
}
func packet(text string, key []byte, now time.Time) ([frameSize]byte, error) {
	var p [frameSize]byte
	if len(key) != 32 {
		return p, ErrKey
	}
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || len(text) > MaxTextBytes {
		return p, ErrText
	}
	copy(p[:4], "GWM!")
	p[4] = 1
	p[5] = byte(len(text))
	binary.BigEndian.PutUint64(p[6:14], uint64(now.Unix()))
	copy(p[14:48], text)
	copy(p[48:], sign(p[:48], key))
	return p, nil
}
func parse(p [frameSize]byte, key []byte) (Result, error) {
	if string(p[:4]) != "GWM!" {
		return Result{}, ErrNotFound
	}
	if p[4] != 1 || p[5] == 0 || int(p[5]) > MaxTextBytes || !hmac.Equal(p[48:], sign(p[:48], key)) {
		return Result{}, ErrInvalid
	}
	text := string(p[14 : 14+int(p[5])])
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" {
		return Result{}, ErrInvalid
	}
	seconds := int64(binary.BigEndian.Uint64(p[6:14]))
	if seconds < 0 {
		return Result{}, ErrInvalid
	}
	return Result{Watermark: text, Timestamp: time.Unix(seconds, 0).UTC().Format(time.RFC3339), Algorithm: Algorithm}, nil
}
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return .299*float64(r>>8) + .587*float64(g>>8) + .114*float64(b>>8)
}
func coefficients(img image.Image, bx, by int) (a, b float64) {
	origin := img.Bounds().Min
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			i := y*8 + x
			l := luminance(img.At(origin.X+bx+x, origin.Y+by+y))
			a += l * basisA[i]
			b += l * basisB[i]
		}
	}
	return
}
func clamp(value float64) uint8 { return uint8(math.Max(0, math.Min(255, math.Round(value)))) }

// Embed returns a separate image; it never mutates its input.
func Embed(img image.Image, text string, key []byte, now time.Time) (*image.NRGBA, error) {
	p, err := packet(text, key, now)
	if err != nil {
		return nil, err
	}
	w, h, err := dimensions(img)
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	origin := img.Bounds().Min
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(img.At(origin.X+x, origin.Y+y)).(color.NRGBA)
			if c.A != 255 {
				return nil, ErrImage
			}
			out.SetNRGBA(x, y, c)
		}
	}
	blocks := (w / 8) * (h / 8)
	repeat := blocks / frameBits
	order := permutation(blocks, key)
	for bit := 0; bit < frameBits; bit++ {
		target := margin
		if p[bit/8]&(1<<uint(7-bit%8)) == 0 {
			target = -margin
		}
		for j := 0; j < repeat; j++ {
			index := order[bit*repeat+j]
			bx, by := (index%(w/8))*8, (index/(w/8))*8
			a, b := coefficients(out, bx, by)
			// Enforce a signed gap; leave already strong evidence untouched.
			if target > 0 && a-b >= target || target < 0 && a-b <= target {
				continue
			}
			delta := (target - (a - b)) / 2
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					i := y*8 + x
					c := out.NRGBAAt(bx+x, by+y)
					d := delta * (basisA[i] - basisB[i])
					out.SetNRGBA(bx+x, by+y, color.NRGBA{clamp(float64(c.R) + d), clamp(float64(c.G) + d), clamp(float64(c.B) + d), 255})
				}
			}
		}
	}
	return out, nil
}
func Extract(img image.Image, key []byte) (Result, error) {
	if len(key) != 32 {
		return Result{}, ErrKey
	}
	w, h, err := dimensions(img)
	if err != nil {
		return Result{}, err
	}
	blocks := (w / 8) * (h / 8)
	repeat := blocks / frameBits
	order := permutation(blocks, key)
	var p [frameSize]byte
	for bit := 0; bit < frameBits; bit++ {
		vote := 0.0
		for j := 0; j < repeat; j++ {
			index := order[bit*repeat+j]
			a, b := coefficients(img, (index%(w/8))*8, (index/(w/8))*8)
			vote += math.Max(-margin, math.Min(margin, a-b))
		}
		if vote > 0 {
			p[bit/8] |= 1 << uint(7-bit%8)
		}
	}
	return parse(p, key)
}
