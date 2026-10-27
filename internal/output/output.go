// Package output encodes and verifies the exact delivered bytes using standard codecs.
package output

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"time"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/quality"
)

var ErrOptions = errors.New("output_options")
var ErrVerification = errors.New("output_verification")

type Options struct {
	Format      string `json:"format"`
	JPEGQuality int    `json:"jpeg_quality,omitempty"`
}

func Normalize(p *Options) (*Options, error) {
	if p == nil {
		return nil, nil
	}
	v := *p
	if v.Format == "" {
		v.Format = "png"
	}
	if v.Format == "png" {
		if v.JPEGQuality != 0 {
			return nil, ErrOptions
		}
		return nil, nil
	}
	if v.Format != "jpeg" {
		return nil, ErrOptions
	}
	if v.JPEGQuality == 0 {
		v.JPEGQuality = 90
	}
	if v.JPEGQuality < 60 || v.JPEGQuality > 100 {
		return nil, ErrOptions
	}
	return &v, nil
}
func Format(p *Options) string {
	if p != nil && p.Format == "jpeg" {
		return "jpeg"
	}
	return "png"
}
func Extension(p *Options) string {
	if Format(p) == "jpeg" {
		return "jpg"
	}
	return "png"
}
func MIME(p *Options) string {
	if Format(p) == "jpeg" {
		return "image/jpeg"
	}
	return "image/png"
}

type limitedBuffer struct{ data bytes.Buffer }

func (b *limitedBuffer) Len() int      { return b.data.Len() }
func (b *limitedBuffer) Bytes() []byte { return b.data.Bytes() }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 50<<20 {
		return 0, io.ErrShortBuffer
	}
	return b.data.Write(p)
}
func Encode(input, marked, original image.Image, key []byte, text string, at time.Time, options *Options, policy quality.Policy) ([]byte, quality.Summary, error) {
	options, err := Normalize(options)
	if err != nil {
		return nil, quality.Summary{}, err
	}
	if !policy.Valid() {
		return nil, quality.Summary{}, quality.ErrPolicy
	}
	var data limitedBuffer
	if Format(options) == "jpeg" {
		err = jpeg.Encode(&data, marked, &jpeg.Options{Quality: options.JPEGQuality})
	} else {
		err = png.Encode(&data, marked)
	}
	if err != nil {
		return nil, quality.Summary{}, err
	}
	decoded, _, err := image.Decode(bytes.NewReader(data.Bytes()))
	if err != nil {
		return nil, quality.Summary{}, err
	}
	summary := quality.Compare(input, marked, original, quality.Policy{})
	summary.Verified = false
	summary.Accepted = false
	enc, final := quality.Measure(marked, decoded), quality.Measure(input, decoded)
	summary.Encoding = &enc
	summary.Final = &final
	digest := sha256.Sum256(data.Bytes())
	summary.OutputSHA256 = hex.EncodeToString(digest[:])
	summary.OutputBytes = int64(data.Len())
	summary.Format = Format(options)
	if options != nil {
		summary.JPEGQuality = options.JPEGQuality
	}
	actual, e := blind.Extract(decoded, key)
	if e != nil || actual.Watermark != text || actual.Timestamp != at.UTC().Format(time.RFC3339) {
		return nil, summary, ErrVerification
	}
	summary.Verified = true
	summary.Accepted = policy.Accept(final)
	if !summary.Accepted {
		return nil, summary, quality.ErrThreshold
	}
	return data.Bytes(), summary, nil
}
