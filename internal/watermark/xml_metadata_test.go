package watermark

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestXMLMetadataExactTextRecordedTimeAndUpsert(t *testing.T) {
	original := []byte(`<w:document xmlns:w="urn:test"><w:body><w:p>A&amp;B unchanged</w:p></w:body></w:document>`)
	body := `A&B -- 第1次 <note> "quote"`
	at := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	marked, err := SetXMLMetadata(original, body, at)
	if err != nil {
		t.Fatal(err)
	}
	text, stamp, found, err := ReadXMLMetadata(marked)
	if err != nil || !found || text != body || stamp != "2001-02-03T04:05:06Z" {
		t.Fatal(text, stamp, found, err)
	}
	if !bytes.Contains(marked, []byte(`<w:body><w:p>A&amp;B unchanged</w:p></w:body>`)) {
		t.Fatal("document content changed")
	}
	again, err := SetXMLMetadata(marked, "new body", at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(again, []byte(metadataPrefix)) != 1 {
		t.Fatal("metadata accumulated")
	}
	text, stamp, found, err = ReadXMLMetadata(again)
	if err != nil || !found || text != "new body" || stamp != "2001-02-03T05:05:06Z" {
		t.Fatal(text, stamp, err)
	}
	for _, raw := range [][]byte{[]byte(`<r/>`), []byte(`<p:r xmlns:p="urn:test"/>`), append([]byte{0xef, 0xbb, 0xbf}, original...)} {
		marked, err = SetXMLMetadata(raw, body, at)
		if err != nil {
			t.Fatal(string(raw), err)
		}
		text, _, found, err = ReadXMLMetadata(marked)
		if err != nil || !found || text != body {
			t.Fatal(text, found, err)
		}
	}
}
func TestXMLMetadataRejectsMalformedOrAmbiguousData(t *testing.T) {
	for _, raw := range []string{`<a><b></a>`, `<a/><b/>`, `<a><!-- WatermarkDataV2:invalid --></a>`, `<a><!-- WatermarkDataV2:invalid --><!-- WatermarkDataV2:other --></a>`} {
		_, _, _, err := ReadXMLMetadata([]byte(raw))
		if !errors.Is(err, ErrXMLMetadata) {
			t.Fatal(raw, err)
		}
	}
}
