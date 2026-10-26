// Package workbench evaluates independent copies without changing the watermark protocol.
package workbench

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"time"
	"watermark-tool/internal/blind"
)

type Case struct {
	ControlPassed *bool    `json:"control_passed,omitempty"`
	Verified      bool     `json:"verified"`
	ID            string   `json:"case_id"`
	Parameters    string   `json:"parameters"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	Recovered     bool     `json:"recovered"`
	Expected      bool     `json:"expected_recovery"`
	Error         string   `json:"error_code,omitempty"`
	Milliseconds  int64    `json:"elapsed_ms"`
	PSNR          *float64 `json:"psnr_db"`
	SHA256        string   `json:"output_sha256"`
	Preview       string   `json:"preview,omitempty"`
}
type Report struct {
	Kind           string `json:"kind"`
	Schema         int    `json:"schema_version"`
	Algorithm      string `json:"algorithm"`
	Source         string `json:"source_sha256"`
	KeyID          string `json:"key_id,omitempty"`
	TestedAt       string `json:"tested_at"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	BaselineOK     bool   `json:"baseline_ok"`
	OriginalIntact bool   `json:"original_intact"`
	Cases          []Case `json:"cases"`
}

func Digest(data []byte) string { s := sha256.Sum256(data); return hex.EncodeToString(s[:]) }
func Code(err error) string {
	for _, e := range []error{blind.ErrNotFound, blind.ErrInvalid, blind.ErrCapacity, blind.ErrImage, blind.ErrText, blind.ErrKey} {
		if errors.Is(err, e) {
			return e.Error()
		}
	}
	return "requestError"
}

// Transform always allocates a fresh image. Resize uses nearest-neighbor sampling.
func Transform(src image.Image, mode string) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	nw, nh := w, h
	ox, oy := 0, 0
	switch mode {
	case "scale-75":
		nw, nh = w*3/4, h*3/4
	case "scale-50":
		nw, nh = w/2, h/2
	case "crop-90":
		nw, nh = w*9/10, h*9/10
		ox, oy = (w-nw)/2, (h-nh)/2
	case "rotate-90":
		nw, nh = h, w
	}
	out := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			sx, sy := x, y
			switch mode {
			case "scale-75", "scale-50":
				sx, sy = x*w/nw, y*h/nh
			case "crop-90":
				sx, sy = x+ox, y+oy
			case "rotate-90":
				sx, sy = y, h-1-x
			}
			out.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return out
}

// PSNR returns nil for different dimensions or identical pixels (infinite PSNR).
func PSNR(a, b image.Image) *float64 {
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return nil
	}
	var sum float64
	for y := 0; y < ab.Dy(); y++ {
		for x := 0; x < ab.Dx(); x++ {
			ac := color.NRGBAModel.Convert(a.At(ab.Min.X+x, ab.Min.Y+y)).(color.NRGBA)
			bc := color.NRGBAModel.Convert(b.At(bb.Min.X+x, bb.Min.Y+y)).(color.NRGBA)
			for _, d := range []float64{float64(ac.R) - float64(bc.R), float64(ac.G) - float64(bc.G), float64(ac.B) - float64(bc.B)} {
				sum += d * d
			}
		}
	}
	if sum == 0 {
		return nil
	}
	v := 10 * math.Log10(255*255/(sum/float64(ab.Dx()*ab.Dy()*3)))
	return &v
}
func preview(src image.Image) string {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > 640 || h > 640 {
		factor := math.Min(640/float64(w), 640/float64(h))
		w, h = int(float64(w)*factor), int(float64(h)*factor)
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.Set(x, y, src.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
		}
	}
	var data bytes.Buffer
	_ = jpeg.Encode(&data, out, &jpeg.Options{Quality: 80})
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
}
func Run(ctx context.Context, src image.Image, text string, key []byte, now time.Time) (Report, error) {
	report := Report{Kind: "experiment", Schema: 1, Algorithm: blind.Algorithm, TestedAt: now.UTC().Format(time.RFC3339), Width: src.Bounds().Dx(), Height: src.Bounds().Dy()}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	baseline, err := blind.Embed(src, text, key, now)
	if err != nil {
		return report, err
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	result, err := blind.Extract(baseline, key)
	if err != nil {
		return report, err
	}
	if result.Watermark != text {
		return report, blind.ErrInvalid
	}
	report.BaselineOK = true
	wrong := make([]byte, 32)
	if _, err = rand.Read(wrong); err != nil {
		return report, err
	}
	for _, mode := range []string{"png", "jpeg-95", "jpeg-85", "jpeg-75", "scale-75", "scale-50", "crop-90", "rotate-90", "unmarked", "wrong-key"} {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		started := time.Now()
		img := image.Image(baseline)
		verifyKey := key
		expected := true
		if mode == "unmarked" {
			img = src
			expected = false
		}
		if mode == "wrong-key" {
			verifyKey = wrong
			expected = false
		}
		img = Transform(img, mode)
		var encoded bytes.Buffer
		quality := 0
		switch mode {
		case "jpeg-95":
			quality = 95
		case "jpeg-85":
			quality = 85
		case "jpeg-75":
			quality = 75
		}
		if quality > 0 {
			err = jpeg.Encode(&encoded, img, &jpeg.Options{Quality: quality})
			if err == nil {
				img, err = jpeg.Decode(bytes.NewReader(encoded.Bytes()))
			}
		} else {
			err = png.Encode(&encoded, img)
		}
		if err != nil {
			return report, err
		}
		found, e := blind.Extract(img, verifyKey)
		row := Case{ID: mode, Parameters: mode, Width: img.Bounds().Dx(), Height: img.Bounds().Dy(), Expected: expected, Recovered: e == nil && found.Watermark == text, SHA256: Digest(encoded.Bytes())}
		row.Verified = e == nil
		if !expected {
			passed := e != nil
			row.ControlPassed = &passed
		}
		if e != nil {
			row.Error = Code(e)
		} else if found.Watermark != text {
			row.Error = "payload_mismatch"
		}
		if expected {
			row.PSNR = PSNR(src, img)
			row.Preview = preview(img)
		}
		row.Milliseconds = time.Since(started).Milliseconds()
		report.Cases = append(report.Cases, row)
	}
	return report, nil
}
