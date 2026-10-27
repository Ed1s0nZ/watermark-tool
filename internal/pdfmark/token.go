package pdfmark

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func pageID(document string, page int) string { return fmt.Sprintf("%s.p%03d", document, page) }
func markerValid(m Marker) bool {
	return m.Version == 1 && idPattern.MatchString(m.DocumentID) && m.ID == pageID(m.DocumentID, m.Page) && keyPattern.MatchString(m.KeyID) && m.Page >= 1 && m.Page <= m.Pages && m.Pages <= MaxPages && validText(m.Text) && digestPattern.MatchString(m.SourceSHA256) && !m.CreatedAt.IsZero()
}
func signature(raw, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte("watermark/pdf-page/v1\x00"))
	h.Write(raw)
	return h.Sum(nil)
}
func encodeMarker(m Marker, key []byte) (string, error) {
	if len(key) != 32 || !markerValid(m) {
		return "", ErrInvalid
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return "", ErrInvalid
	}
	token := "WMPDF1." + base64.RawURLEncoding.EncodeToString(raw) + "." + hex.EncodeToString(signature(raw, key))
	if len(token) > MaxTokenBytes {
		return "", ErrLimit
	}
	return token, nil
}
func decodeMarker(token string, keys KeyResolver) (*Marker, string) {
	if len(token) > MaxTokenBytes {
		return nil, "malformed"
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "WMPDF1" || len(parts[2]) != 64 {
		return nil, "malformed"
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(raw) > 6<<10 {
		return nil, "malformed"
	}
	mac, err := hex.DecodeString(parts[2])
	if err != nil {
		return nil, "malformed"
	}
	var marker Marker
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&marker) != nil || decoder.Decode(new(any)) != io.EOF || !markerValid(marker) {
		return nil, "malformed"
	}
	if keys == nil {
		return nil, "unknown_key"
	}
	key, ok := keys(marker.KeyID)
	if !ok || len(key) != 32 {
		return nil, "unknown_key"
	}
	if !hmac.Equal(mac, signature(raw, key)) {
		return nil, "invalid_marker"
	}
	return &marker, "verified_marker"
}
