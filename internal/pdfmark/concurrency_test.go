package pdfmark

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestIndependentConcurrentDocuments(t *testing.T) {
	input, originalBytes := original(t)
	directory := t.TempDir()
	var workers sync.WaitGroup
	failures := make(chan error, 4)
	for index := 0; index < 4; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			p := params()
			p.DocumentID = fmt.Sprintf("%032x", index+1)
			output := filepath.Join(directory, fmt.Sprintf("marked-%d.pdf", index))
			report, err := Protect(t.Context(), input, output, p, testKey)
			if err == nil && report.Pages[1].Marker.DocumentID != p.DocumentID {
				err = fmt.Errorf("document identity crossed workers")
			}
			failures <- err
		}(index)
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(originalBytes, after) {
		t.Fatal("concurrent workers altered original")
	}
}
