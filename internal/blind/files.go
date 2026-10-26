package blind

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func DecodeKey(value string) ([]byte, error) {
	key, err := hex.DecodeString(value)
	if err != nil || len(key) != 32 {
		return nil, ErrKey
	}
	return key, nil
}

// Load checks encoded size and dimensions before allocating a decoded image.
func Load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImage, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > 50<<20 {
		return nil, ErrImage
	}
	config, format, err := image.DecodeConfig(f)
	if err != nil || (format != "png" && format != "jpeg") || config.Width <= 0 || config.Height <= 0 || config.Width > MaxPixels/config.Height {
		return nil, ErrImage
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, ErrImage
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, ErrImage
	}
	return img, nil
}

// SaveNew refuses every existing output, including symlinks and the input itself.
func SaveNew(path string, img image.Image) (err error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return ErrImage
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(path)
		}
	}()
	if ext == ".png" {
		return png.Encode(f, img)
	}
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
}
func AddFile(input, output, text string, key []byte) error {
	img, err := Load(input)
	if err != nil {
		return err
	}
	out, err := Embed(img, text, key, time.Now())
	if err != nil {
		return err
	}
	return SaveNew(output, out)
}
func ExtractFile(input string, key []byte) (Result, error) {
	img, err := Load(input)
	if err != nil {
		return Result{}, err
	}
	return Extract(img, key)
}
