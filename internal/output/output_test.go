package output

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/quality"
)

func TestExactDeliveredBytesAndFinalGate(t *testing.T) {
	input, err := blind.Load("../blind/testdata/clean.png")
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{7}, 32)
	at := time.Now().UTC().Truncate(time.Second)
	marked, err := blind.Embed(input, "export-image", key, at)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []int{60, 75, 90, 100} {
		data, summary, e := Encode(input, marked, nil, key, "export-image", at, &Options{Format: "jpeg", JPEGQuality: q}, quality.Policy{})
		t.Log(q, summary.Verified, summary.Embedding, summary.Final, e)
		if e != nil {
			if !errors.Is(e, ErrVerification) || summary.Verified || summary.Accepted || summary.OutputBytes == 0 || !summary.Valid() {
				t.Fatal(summary, e)
			}
			continue
		}
		if len(data) < 2 || data[0] != 255 || data[1] != 216 || !summary.Valid() {
			t.Fatal("JPEG signature", summary)
		}
		decoded, e := jpeg.Decode(bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		r, e := blind.Extract(decoded, key)
		if e != nil || r.Watermark != "export-image" {
			t.Fatal("actual JPEG", e)
		}
		digest := sha256.Sum256(data)
		if summary.OutputSHA256 != hex.EncodeToString(digest[:]) || summary.OutputBytes != int64(len(data)) {
			t.Fatal("actual digest")
		}
	}
	data, pngSummary, err := Encode(input, marked, input, key, "export-image", at, nil, quality.Policy{})
	if err != nil || !bytes.HasPrefix(data, []byte{137, 80, 78, 71}) || pngSummary.Encoding.Status != "infinite" {
		t.Fatal(pngSummary, err)
	}
	// JPEG encoding can cross an explicit threshold even though raw embedding passes.
	_, jpgSummary, err := Encode(input, marked, nil, key, "export-image", at, &Options{Format: "jpeg", JPEGQuality: 100}, quality.Policy{})
	if err != nil {
		t.Fatal("high-quality fixture must verify", err)
	}
	if jpgSummary.Final.DB == nil || jpgSummary.Embedding.DB == nil || *jpgSummary.Final.DB >= *jpgSummary.Embedding.DB {
		t.Fatal("fixture needs measurable encoding loss", jpgSummary)
	}
	threshold := (*jpgSummary.Final.DB + *jpgSummary.Embedding.DB) / 2
	if !(quality.Policy{MinimumPSNR: &threshold}).Accept(jpgSummary.Embedding) {
		t.Fatal("pre-encoding control")
	}
	rejected, summary, err := Encode(input, marked, nil, key, "export-image", at, &Options{Format: "jpeg", JPEGQuality: 100}, quality.Policy{MinimumPSNR: &threshold})
	if !errors.Is(err, quality.ErrThreshold) || len(rejected) != 0 || summary.Accepted || !summary.Verified {
		t.Fatal(summary, err)
	}
	// An intentionally damaged marked image must never be delivered as valid.
	damaged := image.NewNRGBA(marked.Bounds())
	for y := 0; y < damaged.Bounds().Dy(); y++ {
		for x := 0; x < damaged.Bounds().Dx(); x++ {
			damaged.SetNRGBA(x, y, color.NRGBA{128, 128, 128, 255})
		}
	}
	if b, s, e := Encode(input, damaged, nil, key, "export-image", at, &Options{Format: "jpeg", JPEGQuality: 60}, quality.Policy{}); !errors.Is(e, ErrVerification) || b != nil || s.Verified {
		t.Fatal("bad final delivered", e)
	}
	for _, o := range []*Options{{Format: "gif"}, {Format: "jpeg", JPEGQuality: 59}, {Format: "jpeg", JPEGQuality: 101}, {Format: "png", JPEGQuality: 90}} {
		if _, e := Normalize(o); e == nil {
			t.Fatal(o)
		}
	}
}
