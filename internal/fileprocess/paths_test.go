package fileprocess

import (
	"reflect"
	"strings"
	"testing"
	"watermark-tool/internal/filebundle"
)

func TestOutputPathsCollisionsAndDirectoryPreservation(t *testing.T) {
	a, b, c := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 32)
	for _, sources := range [][]filebundle.Record{
		{{ID: a, RelativePath: "folder/photo.png"}, {ID: b, RelativePath: "folder/PHOTO.jpg"}},
		{{ID: a, RelativePath: "folder/photo.png"}, {ID: b, RelativePath: "folder/photo.jpg/inner.docx"}},
		{{ID: a, RelativePath: "folder/é.png"}, {ID: b, RelativePath: "folder/e\u0301.jpg"}},
	} {
		plans := map[string]Plan{a: {OutputExtension: "jpg"}}
		first, err := OutputPaths(sources, plans)
		if err != nil {
			t.Fatal(err)
		}
		second, err := OutputPaths(sources, plans)
		if err != nil || !reflect.DeepEqual(first, second) {
			t.Fatal("not deterministic", err)
		}
		if !strings.HasPrefix(first[a], "folder/") || first[a] == first[b] || !strings.Contains(first[a], "~"+a) {
			t.Fatal(first)
		}
	}
	paths, err := OutputPaths([]filebundle.Record{{ID: c, RelativePath: "report.json"}}, nil)
	if err != nil || paths[c] != "report.json" {
		t.Fatal(paths, err)
	} // archive caller prefixes files/, keeping the manifest separate.
}
func TestOutputPathsLongUTF8AndInvalidInputs(t *testing.T) {
	a, b := strings.Repeat("a", 32), strings.Repeat("b", 32)
	base := strings.Repeat("图", 80)
	sources := []filebundle.Record{{ID: a, RelativePath: base + ".png"}, {ID: b, RelativePath: base + ".jpg"}}
	paths, err := OutputPaths(sources, map[string]Plan{a: {OutputExtension: "jpg"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range paths {
		if _, err = filebundle.SafePath(name); err != nil {
			t.Fatal(err)
		}
	}
	for _, sources := range [][]filebundle.Record{{{ID: a, RelativePath: "../bad"}}, {{ID: a, RelativePath: "ok.png"}, {ID: a, RelativePath: "other.png"}}, {{ID: a, RelativePath: strings.Repeat("x", 256) + ".png"}}} {
		if _, err = OutputPaths(sources, nil); err == nil {
			t.Fatal("invalid names accepted")
		}
	}
}
