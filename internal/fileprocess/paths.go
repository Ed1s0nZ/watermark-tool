package fileprocess

import (
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"path"
	"strings"
	"unicode/utf8"
	"watermark-tool/internal/filebundle"
)

// OutputPaths computes every rename before execution. Returned paths are relative
// to the archive's files/ root; reports live outside that root.
func OutputPaths(sources []filebundle.Record, plans map[string]Plan) (map[string]string, error) {
	return OutputPathsWithDirectories(sources, plans, nil)
}

// OutputPathsWithDirectories also reserves explicit empty directories from ZIP input.
func OutputPathsWithDirectories(sources []filebundle.Record, plans map[string]Plan, directories []string) (map[string]string, error) {
	if len(sources) < 1 || len(sources) > 100 {
		return nil, ErrOptions
	}
	names := map[string]string{}
	seen := map[string]bool{}
	for _, source := range sources {
		if _, err := filebundle.SafePath(source.RelativePath); err != nil {
			return nil, err
		}
		if !identityPattern.MatchString(source.ID) || seen[source.ID] {
			return nil, ErrSource
		}
		seen[source.ID] = true
		name := source.RelativePath
		if p, ok := plans[source.ID]; ok {
			switch p.OutputExtension {
			case "png", "jpg", "jpeg", "pdf", "docx", "pptx", "xlsx", "rtf", "odt":
			default:
				return nil, ErrOptions
			}
			name = strings.TrimSuffix(name, path.Ext(name)) + "." + p.OutputExtension
		}
		if _, err := filebundle.SafePath(name); err != nil {
			return nil, err
		}
		names[source.ID] = name
	}
	canonical := func(name string) string { return cases.Fold().String(norm.NFC.String(name)) }
	for attempt := 0; attempt <= len(sources); attempt++ {
		counts := map[string]int{}
		parents := map[string]bool{}
		for _, dir := range directories {
			if _, err := filebundle.SafePath(dir); err != nil {
				return nil, err
			}
			parents[canonical(dir)] = true
		}
		for _, name := range names {
			counts[canonical(name)]++
			for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
				parents[canonical(parent)] = true
			}
		}
		conflicts := []string{}
		for _, source := range sources {
			key := canonical(names[source.ID])
			if counts[key] > 1 || parents[key] {
				conflicts = append(conflicts, source.ID)
			}
		}
		if len(conflicts) == 0 {
			return names, nil
		}
		for _, id := range conflicts {
			name := names[id]
			ext := path.Ext(name)
			suffix := fmt.Sprintf("~%s-%d%s", id, attempt+1, ext)
			dir := path.Dir(name)
			prefix := ""
			if dir != "." {
				prefix = dir + "/"
			}
			max := min(255-len(suffix), 512-len(prefix)-len(suffix))
			if max < 1 {
				return nil, filebundle.ErrPath
			}
			base := strings.TrimSuffix(path.Base(name), ext)
			if len(base) > max {
				base = base[:max]
				for !utf8.ValidString(base) {
					base = base[:len(base)-1]
				}
			}
			base = strings.TrimRight(base, " .")
			if base == "" {
				base = "f"
			}
			name = prefix + base + suffix
			if _, err := filebundle.SafePath(name); err != nil {
				return nil, err
			}
			names[id] = name
		}
	}
	return nil, filebundle.ErrPath
}
