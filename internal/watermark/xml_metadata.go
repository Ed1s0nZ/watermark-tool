package watermark

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrXMLMetadata = errors.New("invalid_xml_watermark_metadata")

const metadataPrefix = " WatermarkDataV2:"

type xmlMetadata struct {
	Version int    `json:"version"`
	Text    string `json:"text"`
	AddedAt string `json:"added_at"`
}
type xmlCut struct{ start, end int }
type xmlScan struct {
	cuts        []xmlCut
	comments    [][]byte
	close, end  int
	selfClosing bool
	name        string
}

func xmlQName(name xml.Name) string {
	if name.Space != "" {
		return name.Space + ":" + name.Local
	}
	return name.Local
}

// scanXML tracks token offsets so existing document bytes and namespaces need not be reserialized.
func scanXML(data []byte) (xmlScan, error) {
	result := xmlScan{}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	stack := []string{}
	rootSeen := false
	for {
		before := int(decoder.InputOffset())
		token, err := decoder.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, ErrXMLMetadata
		}
		after := int(decoder.InputOffset())
		switch token := token.(type) {
		case xml.StartElement:
			if len(stack) == 0 {
				if rootSeen {
					return result, ErrXMLMetadata
				}
				rootSeen = true
				result.name = xmlQName(token.Name)
			}
			stack = append(stack, xmlQName(token.Name))
			if len(stack) > 128 {
				return result, ErrXMLMetadata
			}
		case xml.EndElement:
			if len(stack) == 0 || stack[len(stack)-1] != xmlQName(token.Name) {
				return result, ErrXMLMetadata
			}
			if len(stack) == 1 {
				result.close = before
				result.end = after
				result.selfClosing = before == after
			}
			stack = stack[:len(stack)-1]
		case xml.Comment:
			if len(stack) > 0 && bytes.HasPrefix(token, []byte(metadataPrefix)) {
				if len(token) > 4096 {
					return result, ErrXMLMetadata
				}
				result.comments = append(result.comments, append([]byte{}, token...))
				result.cuts = append(result.cuts, xmlCut{before, after})
			}
		case xml.CharData:
			rawText := []byte(token)
			if before == 0 && !rootSeen {
				rawText = bytes.TrimPrefix(rawText, []byte{0xef, 0xbb, 0xbf})
			}
			if len(stack) == 0 && len(bytes.TrimSpace(rawText)) > 0 {
				return result, ErrXMLMetadata
			}
		}
	}
	if !rootSeen || len(stack) != 0 || result.end == 0 {
		return result, ErrXMLMetadata
	}
	return result, nil
}

// SetXMLMetadata stores declared compatibility metadata, not a cryptographic document signature.
func SetXMLMetadata(data []byte, text string, at time.Time) ([]byte, error) {
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || len(text) > 100 || at.IsZero() {
		return nil, ErrXMLMetadata
	}
	scan, err := scanXML(data)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(xmlMetadata{Version: 2, Text: text, AddedAt: at.UTC().Format(time.RFC3339)})
	comment := []byte("<!--" + metadataPrefix + base64.StdEncoding.EncodeToString(payload) + " -->")
	var out bytes.Buffer
	cursor := 0
	for _, cut := range scan.cuts {
		out.Write(data[cursor:cut.start])
		cursor = cut.end
	}
	if scan.selfClosing {
		position := scan.end - 2
		if position < cursor || string(data[position:scan.end]) != "/>" {
			return nil, ErrXMLMetadata
		}
		out.Write(data[cursor:position])
		out.WriteByte('>')
		out.Write(comment)
		out.WriteString("</" + scan.name + ">")
		out.Write(data[scan.end:])
	} else {
		if scan.close < cursor {
			return nil, ErrXMLMetadata
		}
		out.Write(data[cursor:scan.close])
		out.Write(comment)
		out.Write(data[scan.close:])
	}
	return out.Bytes(), nil
}
func ReadXMLMetadata(data []byte) (text, addedAt string, found bool, err error) {
	scan, err := scanXML(data)
	if err != nil {
		return "", "", false, err
	}
	if len(scan.comments) == 0 {
		return "", "", false, nil
	}
	if len(scan.comments) != 1 {
		return "", "", true, ErrXMLMetadata
	}
	encoded := bytes.TrimSpace(bytes.TrimPrefix(scan.comments[0], []byte(metadataPrefix)))
	raw, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", "", true, ErrXMLMetadata
	}
	var metadata xmlMetadata
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&metadata) != nil || decoder.Decode(new(any)) != io.EOF || metadata.Version != 2 || !utf8.ValidString(metadata.Text) || metadata.Text == "" || len(metadata.Text) > 100 {
		return "", "", true, ErrXMLMetadata
	}
	at, err := time.Parse(time.RFC3339, metadata.AddedAt)
	if err != nil {
		return "", "", true, ErrXMLMetadata
	}
	return metadata.Text, at.UTC().Format(time.RFC3339), true, nil
}
