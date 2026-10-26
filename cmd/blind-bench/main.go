// blind-bench runs independent image cases and never writes to the source images.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"watermark-tool/internal/blind"
)

type source struct{ Name, URL, SHA256 string }

var sources = []source{
	{"flower.jpg", "https://upload.wikimedia.org/wikipedia/commons/3/3f/JPEG_example_flower.jpg", "c160fa1bc488af7f48984bd627bc5cbc5a20b98eaaf11af92e99a206b376252d"},
	{"astronaut.png", "https://raw.githubusercontent.com/scikit-image/scikit-image/v0.19.3/skimage/data/astronaut.png", "88431cd9653ccd539741b555fb0a46b61558b301d4110412b5bc28b5e3ea6cb5"},
	{"coffee.png", "https://raw.githubusercontent.com/scikit-image/scikit-image/v0.19.3/skimage/data/coffee.png", "cc02f8ca188b167c775a7101b5d767d1e71792cf762c33d6fa15a4599b5a8de7"},
}

type record struct {
	Name           string    `json:"name"`
	Source         string    `json:"source"`
	SHA256         string    `json:"sha256"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	OriginalIntact bool      `json:"original_intact"`
	Cases          []outcome `json:"cases"`
}
type outcome struct {
	Mode         string   `json:"mode"`
	Recovered    bool     `json:"recovered"`
	Error        string   `json:"error,omitempty"`
	PSNR         *float64 `json:"psnr_db,omitempty"`
	Milliseconds int64    `json:"milliseconds"`
}

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func psnr(a, b image.Image) float64 {
	var mse float64
	r := a.Bounds()
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			ar, ag, ab, _ := a.At(r.Min.X+x, r.Min.Y+y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			for _, d := range []float64{float64(ar>>8) - float64(br>>8), float64(ag>>8) - float64(bg>>8), float64(ab>>8) - float64(bb>>8)} {
				mse += d * d
			}
		}
	}
	mse /= float64(r.Dx() * r.Dy() * 3)
	if mse == 0 {
		return 100
	}
	return 10 * math.Log10(255*255/mse)
}
func download(path, url string) error {
	if _, err := os.Stat(path); err == nil {
		return os.Chmod(path, 0444)
	}
	client := http.Client{Timeout: 60 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("download %s: %d", url, response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 50<<20+1))
	if err != nil {
		return err
	}
	if len(data) > 50<<20 {
		return fmt.Errorf("source too large")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func run(base string) error {
	originals := filepath.Join(base, "originals")
	if err := os.MkdirAll(originals, 0700); err != nil {
		return err
	}
	runDir, err := os.MkdirTemp(base, "run-")
	if err != nil {
		return err
	}
	keyHash := sha256.Sum256([]byte("public-test-key/go-dct-v1/not-for-production"))
	key := keyHash[:]
	text := "项目-A100"
	var records []record
	criticalFailed := false
	for _, s := range sources {
		path := filepath.Join(originals, s.Name)
		if err := download(path, s.URL); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		before := hash(data)
		if before != s.SHA256 {
			return fmt.Errorf("source digest mismatch: %s", s.Name)
		}
		img, err := blind.Load(path)
		if err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
		rec := record{Name: s.Name, Source: s.URL, SHA256: before, Width: img.Bounds().Dx(), Height: img.Bounds().Dy()}
		for _, quality := range []int{0, 95, 85, 75} {
			mode := "png-reencode"
			if quality > 0 {
				mode = fmt.Sprintf("jpeg-%d", quality)
			}
			dir := filepath.Join(runDir, s.Name+"-"+mode)
			if err := os.MkdirAll(dir, 0700); err != nil {
				return err
			}
			// Every case starts from a byte-for-byte fresh original copy.
			input := filepath.Join(dir, "input"+filepath.Ext(s.Name))
			if err := os.WriteFile(input, data, 0600); err != nil {
				return err
			}
			copyData, _ := os.ReadFile(input)
			if hash(copyData) != before {
				return fmt.Errorf("copy mismatch")
			}
			start := time.Now()
			output := filepath.Join(dir, "watermarked.png")
			inputImage, err := blind.Load(input)
			if err != nil {
				return err
			}
			marked, err := blind.Embed(inputImage, text, key, time.Unix(1793000000, 0))
			if err != nil {
				return err
			}
			if err := blind.SaveNew(output, marked); err != nil {
				return err
			}
			watermarked, err := blind.Load(output)
			if err != nil {
				return err
			}
			var encoded bytes.Buffer
			if quality == 0 {
				err = png.Encode(&encoded, watermarked)
			} else {
				err = jpeg.Encode(&encoded, watermarked, &jpeg.Options{Quality: quality})
			}
			if err != nil {
				return err
			}
			ext := ".png"
			if quality > 0 {
				ext = ".jpg"
			}
			attack := filepath.Join(dir, "reencoded"+ext)
			if err := os.WriteFile(attack, encoded.Bytes(), 0600); err != nil {
				return err
			}
			decoded, err := blind.Load(attack)
			if err != nil {
				return err
			}
			result, extractErr := blind.Extract(decoded, key)
			qualityScore := psnr(img, decoded)
			row := outcome{Mode: mode, Recovered: extractErr == nil && result.Watermark == text, PSNR: &qualityScore, Milliseconds: time.Since(start).Milliseconds()}
			if extractErr != nil {
				row.Error = extractErr.Error()
			}
			rec.Cases = append(rec.Cases, row)
			if quality == 0 && !row.Recovered {
				criticalFailed = true
			}
		}
		// Negative controls use unmodified original and a wrong key on a fresh watermarked image.
		cleanStart := time.Now()
		_, cleanErr := blind.Extract(img, key)
		rec.Cases = append(rec.Cases, outcome{Mode: "clean-negative", Recovered: cleanErr != nil, Milliseconds: time.Since(cleanStart).Milliseconds()})
		marked, err := blind.Embed(img, text, key, time.Unix(1793000000, 0))
		if err != nil {
			return err
		}
		wrong := sha256.Sum256([]byte("different-test-key"))
		_, wrongErr := blind.Extract(marked, wrong[:])
		rec.Cases = append(rec.Cases, outcome{Mode: "wrong-key-negative", Recovered: wrongErr != nil})
		if cleanErr == nil || wrongErr == nil {
			criticalFailed = true
		}
		after, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rec.OriginalIntact = hash(after) == before
		if !rec.OriginalIntact {
			return fmt.Errorf("original changed: %s", s.Name)
		}
		records = append(records, rec)
	}
	jsonData, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "report.json"), jsonData, 0600); err != nil {
		return err
	}
	var md bytes.Buffer
	fmt.Fprintf(&md, "# Go DCT v1 real-image benchmark\n\nUTC: %s\n\nPublic benchmark key only; text: %s. Negative rows: true means correctly rejected. PSNR compares decoded output against original decoded RGB. 100 means identical (sentinel).\n\n", time.Now().UTC().Format(time.RFC3339), text)
	for _, rec := range records {
		fmt.Fprintf(&md, "\n## %s\n\n| Image | Size | Case | Pass | PSNR dB | ms | Error |\n|---|---|---|---|---|---|---|\n", rec.Name)
		for _, row := range rec.Cases {
			qualityText := "N/A"
			if row.PSNR != nil {
				qualityText = fmt.Sprintf("%.2f", *row.PSNR)
			}
			fmt.Fprintf(&md, "| %s | %dx%d | %s | %t | %s | %d | %s |\n", rec.Name, rec.Width, rec.Height, row.Mode, row.Recovered, qualityText, row.Milliseconds, row.Error)
		}
		fmt.Fprintf(&md, "\nSource: %s\n\nSHA256: `%s`; original intact: %t\n\n", rec.Source, rec.SHA256, rec.OriginalIntact)
	}
	if err := os.WriteFile(filepath.Join(runDir, "report.md"), md.Bytes(), 0600); err != nil {
		return err
	}
	fmt.Println(runDir)
	if criticalFailed {
		return fmt.Errorf("baseline or negative controls failed; inspect report in %s", runDir)
	}
	return nil
}
func main() {
	dir := flag.String("dir", "/tmp/watermark-go-bench", "External evidence directory (originals and independent cases)")
	flag.Parse()
	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
