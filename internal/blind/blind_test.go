package blind

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(50 + x%150), uint8(60 + y%150), uint8(70 + (x+y)%130), 255})
		}
	}
	return img
}
func TestRoundTripAndReencoding(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	img := fixture()
	before := sha256.Sum256(img.Pix)
	now := time.Unix(1793000000, 0)
	out, err := Embed(img, "项目-A100", key, now)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(img.Pix) != before {
		t.Fatal("input changed")
	}
	for _, quality := range []int{0, 95, 85, 75} {
		t.Run(string(rune('A'+quality)), func(t *testing.T) {
			var buffer bytes.Buffer
			if quality == 0 {
				err = png.Encode(&buffer, out)
			} else {
				err = jpeg.Encode(&buffer, out, &jpeg.Options{Quality: quality})
			}
			if err != nil {
				t.Fatal(err)
			}
			decoded, _, err := image.Decode(&buffer)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Extract(decoded, key)
			if err != nil || result.Watermark != "项目-A100" || result.Timestamp != now.UTC().Format(time.RFC3339) {
				t.Fatalf("quality=%d result=%+v err=%v", quality, result, err)
			}
		})
	}
}
func TestRejectsInvalidInputs(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	img := fixture()
	if _, err := Extract(img, key); err == nil {
		t.Fatal("clean image accepted")
	}
	out, _ := Embed(img, "test", key, time.Now())
	if _, err := Extract(out, bytes.Repeat([]byte{8}, 32)); err == nil {
		t.Fatal("wrong key accepted")
	}
	for _, text := range []string{"", strings.Repeat("a", MaxTextBytes+1), string([]byte{0xff})} {
		if _, err := Embed(img, text, key, time.Now()); !errors.Is(err, ErrText) {
			t.Fatal(err)
		}
	}
	if _, err := Embed(image.NewNRGBA(image.Rect(0, 0, 8, 8)), "test", key, time.Now()); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	img.SetNRGBA(0, 0, color.NRGBA{})
	if _, err := Embed(img, "test", key, time.Now()); !errors.Is(err, ErrImage) {
		t.Fatal(err)
	}
	p, _ := packet("test", key, time.Now())
	p[15] ^= 1
	if _, err := parse(p, key); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := DecodeKey("bad"); !errors.Is(err, ErrKey) {
		t.Fatal(err)
	}
}
func TestFilesNeverOverwrite(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "in.png")
	if err := SaveNew(input, fixture()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(input)
	key := bytes.Repeat([]byte{7}, 32)
	if err := AddFile(input, input, "test", key); err == nil {
		t.Fatal("overwrote input")
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(before, after) {
		t.Fatal("input changed")
	}
	output := filepath.Join(dir, "out.png")
	if err := AddFile(input, output, "test", key); err != nil {
		t.Fatal(err)
	}
	if result, err := ExtractFile(output, key); err != nil || result.Watermark != "test" {
		t.Fatal(result, err)
	}
	os.WriteFile(filepath.Join(dir, "bad.png"), []byte("not an image"), 0600)
	if _, err := Load(filepath.Join(dir, "bad.png")); !errors.Is(err, ErrImage) {
		t.Fatal(err)
	}
}
func TestDCTBasis(t *testing.T) {
	var aa, bb, ab float64
	for i := 0; i < 64; i++ {
		aa += basisA[i] * basisA[i]
		bb += basisB[i] * basisB[i]
		ab += basisA[i] * basisB[i]
	}
	if math.Abs(aa-1) > 1e-10 || math.Abs(bb-1) > 1e-10 || math.Abs(ab) > 1e-10 {
		t.Fatal(aa, bb, ab)
	}
}

var generateV1 = flag.Bool("generate-v1-fixture", false, "Create absent v1 compatibility fixtures; never overwrite")

func TestV1Compatibility(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	now := time.Unix(1793000000, 0)
	if *generateV1 {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		clean := fixture()
		if err := SaveNew("testdata/clean.png", clean); err != nil {
			t.Fatal(err)
		}
		marked, err := Embed(clean, "项目-A100", key, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := SaveNew("testdata/v1.png", marked); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ExtractFile("testdata/v1.png", key)
	if err != nil || result.Watermark != "项目-A100" || result.Timestamp != now.UTC().Format(time.RFC3339) {
		t.Fatal("v1 compatibility broken", result, err)
	}
}

func TestCapacityEdgesAndAliases(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	base := fixture()
	img := base.SubImage(image.Rect(11, 13, 331, 333))
	out, err := Embed(img, strings.Repeat("a", 34), key, time.Unix(1793000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Extract(out, key)
	if err != nil || result.Watermark != strings.Repeat("a", 34) {
		t.Fatal(result, err)
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "in.png")
	if err := SaveNew(input, base); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.png")
	if err := os.Symlink(input, alias); err != nil {
		t.Skip("symlink unavailable")
	}
	before, _ := os.ReadFile(input)
	if err := AddFile(input, alias, "test", key); err == nil {
		t.Fatal("symlink output overwritten")
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(before, after) {
		t.Fatal("original changed via alias")
	}
	if err := AddFile(input, filepath.Join(dir, "out.jpg"), "test", key); err != nil {
		t.Fatal(err)
	}
	if result, err := ExtractFile(filepath.Join(dir, "out.jpg"), key); err != nil || result.Watermark != "test" {
		t.Fatal(result, err)
	}
}
