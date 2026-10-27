package delivery

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
)

// ApprovalInputs verifies the actual private files against the immutable upload identities.
func (m *Manager) ApprovalInputs(id string) (distribution.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.store.Job(id)
	if err != nil {
		return j, err
	}
	for _, source := range j.Sources {
		if err = m.checkInput(j, source.ID, ""); err != nil {
			return j, err
		}
	}
	if j.Mode == "dual" {
		for _, copy := range j.Copies {
			if err = m.checkInput(j, copy.SourceID, copy.ID); err != nil {
				return j, err
			}
		}
	}
	return j, nil
}

// ApprovalThumbnail never exports an original or full-size rendered input.
func (m *Manager) ApprovalThumbnail(id, sourceID, copyID string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.store.Job(id)
	if err != nil {
		return nil, err
	}
	if err = m.checkInput(j, sourceID, copyID); err != nil {
		return nil, err
	}
	input, err := blind.Load(m.inputPath(id, sourceID, copyID))
	if err != nil {
		return nil, err
	}
	bounds := input.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w > 256 || h > 256 {
		if w >= h {
			h = h * 256 / w
			w = 256
		} else {
			w = w * 256 / h
			h = 256
		}
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	thumb := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			thumb.Set(x, y, input.At(bounds.Min.X+x*bounds.Dx()/w, bounds.Min.Y+y*bounds.Dy()/h))
		}
	}
	var out bytes.Buffer
	if err = jpeg.Encode(&out, thumb, &jpeg.Options{Quality: 65}); err != nil {
		return nil, err
	}
	if out.Len() > 128<<10 {
		return nil, distribution.ErrLimit
	}
	return out.Bytes(), nil
}

// eraseDeniedArchive removes an output that lost authorization before its final state commit.
func (m *Manager) eraseDeniedArchive(id string) { _ = os.Remove(m.archivePath(id)) }
