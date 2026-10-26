package workbench

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"math"
	"testing"
	"time"
	"watermark-tool/internal/blind"
)

func TestIndependentTransformsAndQuality(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.NRGBA{uint8(x * 30), uint8(y * 60), 70, 255})
		}
	}
	original := append([]byte(nil), img.Pix...)
	rotated := Transform(img, "rotate-90")
	if rotated.Bounds().Dx() != 2 || rotated.Bounds().Dy() != 4 || rotated.NRGBAAt(0, 0) != img.NRGBAAt(0, 1) {
		t.Fatal("rotation mapping")
	}
	half := Transform(img, "scale-50")
	if half.NRGBAAt(1, 0) != img.NRGBAAt(2, 0) {
		t.Fatal("sampling")
	}
	rotated.SetNRGBA(0, 0, color.NRGBA{})
	if !bytes.Equal(original, img.Pix) {
		t.Fatal("source changed")
	}
	if PSNR(img, half) != nil || PSNR(img, img) != nil {
		t.Fatal("undefined PSNR")
	}
	changed := Transform(img, "png")
	changed.SetNRGBA(0, 0, color.NRGBA{1, 0, 70, 255})
	v := PSNR(img, changed)
	want := 10 * math.Log10(255*255*24)
	if v == nil || math.Abs(*v-want) > 1e-8 {
		t.Fatal(v, want)
	}
}
func TestLabRealRecoveryAndCancellation(t *testing.T) {
	img, err := blind.Load("../blind/testdata/clean.png")
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{9}, 32)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Run(ctx, img, "项目-A", key, time.Now()); err != context.Canceled {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), img, "项目-A", key, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !report.BaselineOK || len(report.Cases) != 10 || !report.Cases[0].Recovered {
		t.Fatal(report)
	}
	for i, row := range report.Cases {
		if len(row.SHA256) != 64 {
			t.Fatal(row)
		}
		if i >= 8 && (row.Recovered || row.Expected || row.ControlPassed == nil || !*row.ControlPassed) {
			t.Fatal("negative control", row)
		}
		if i < 8 && row.Preview == "" {
			t.Fatal("missing preview")
		}
		if i >= 4 && i <= 6 && row.PSNR != nil {
			t.Fatal("different size PSNR")
		}
	}
	// JPEG rows are genuine lossy encodes, not relabelled baseline pixels.
	if report.Cases[0].SHA256 == report.Cases[1].SHA256 {
		t.Fatal("not independently encoded")
	}
}

func TestLabBaselineFailureAndMarkedControl(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	if _, err := Run(context.Background(), image.NewNRGBA(image.Rect(0, 0, 8, 8)), "x", key, time.Now()); err != blind.ErrCapacity {
		t.Fatal(err)
	}
	img, err := blind.Load("../blind/testdata/clean.png")
	if err != nil {
		t.Fatal(err)
	}
	marked, err := blind.Embed(img, "old payload", key, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), marked, "new payload", key, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	row := report.Cases[8]
	if !row.Verified || row.ControlPassed == nil || *row.ControlPassed {
		t.Fatal("pre-marked input hidden", row)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err = Run(ctx, img, "cancel", key, time.Now()); err != context.DeadlineExceeded {
		t.Fatal(err)
	}
}
