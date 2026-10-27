package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/workbench"
)

func buildVerification(input string, store *distribution.Store, requestedKey string) (verificationReport, error) {
	started := time.Now()
	report := verificationReport{Kind: "verification", Schema: 1, TestedAt: started.UTC().Format(time.RFC3339), Algorithm: blind.Algorithm}
	data, err := os.ReadFile(input)
	if err != nil {
		return report, err
	}
	report.Source = workbench.Digest(data)
	img, err := blind.Load(input)
	if err != nil {
		return report, err
	}
	report.Width = img.Bounds().Dx()
	report.Height = img.Bounds().Dy()
	result, keyID, err := verifyWorkbench(img, store, requestedKey)
	switch {
	case err == nil:
		report.Status = "unregistered"
		report.Result = &result
		report.KeyID = keyID
		if store != nil {
			report.Record, err = store.Lookup(result, keyID)
			if err != nil {
				return report, err
			}
			if report.Record != nil {
				report.Status = "registered"
			}
		}
	case errors.Is(err, blind.ErrNotFound):
		report.Status = "not_found"
		report.Error = workbench.Code(err)
	case errors.Is(err, blind.ErrInvalid), errors.Is(err, blind.ErrCapacity):
		report.Status = "invalid"
		report.Error = workbench.Code(err)
	default:
		return report, err
	}
	report.Milliseconds = time.Since(started).Milliseconds()
	return report, nil
}
func registerBatchTrace(api *gin.RouterGroup, store *distribution.Store, tempDir string) {
	api.POST("/workbench/trace-batch", RateLimitMiddleware(), func(c *gin.Context) {
		select {
		case blindSlots <- struct{}{}:
			defer func() { <-blindSlots }()
		default:
			c.JSON(429, gin.H{"code": "blind_busy"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, (128<<20)+(1<<20))
		defer func() {
			if c.Request.MultipartForm != nil {
				_ = c.Request.MultipartForm.RemoveAll()
			}
		}()
		if store == nil {
			if _, _, err := workbenchKey(nil); err != nil {
				platformError(c, err)
				return
			}
		} else {
			if _, err := store.Keys(); err != nil {
				platformError(c, err)
				return
			}
		}
		if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
			c.JSON(413, gin.H{"code": "tooLarge"})
			return
		}
		form := c.Request.MultipartForm
		files := form.File["files"]
		if len(form.File) != 1 || len(form.Value) != 0 || !validTraceFiles(files) {
			c.JSON(422, gin.H{"code": "trace_limit"})
			return
		}
		dir, err := os.MkdirTemp(tempDir, "trace-")
		if err != nil {
			platformError(c, err)
			return
		}
		defer os.RemoveAll(dir)
		var id [16]byte
		if _, err = rand.Read(id[:]); err != nil {
			platformError(c, err)
			return
		}
		batch := hex.EncodeToString(id[:])
		c.Set("audit_target", batch)
		c.Header("Content-Type", "application/x-ndjson")
		c.Header("Cache-Control", "no-store")
		c.Header("X-Accel-Buffering", "no")
		write := func(v any) bool {
			if c.Request.Context().Err() != nil {
				return false
			}
			if err := json.NewEncoder(c.Writer).Encode(v); err != nil {
				return false
			}
			c.Writer.Flush()
			return true
		}
		c.Set("audit_outcome", "cancelled")
		if !write(gin.H{"type": "begin", "batch_id": batch, "total": len(files)}) {
			return
		}
		seen := map[string]int{}
		partial := false
		for i, f := range files {
			if c.Request.Context().Err() != nil {
				return
			}
			path := filepath.Join(dir, "input")
			if err = c.SaveUploadedFile(f, path); err != nil {
				c.Set("audit_outcome", "failed")
				c.Set("audit_code", "trace_io")
				write(gin.H{"type": "fatal", "code": "requestError"})
				return
			}
			report, e := buildVerification(path, store, "")
			_ = os.Remove(path)
			row := gin.H{"type": "item", "index": i + 1, "name": f.Filename, "report": report}
			if first := seen[report.Source]; first > 0 {
				row["duplicate_of"] = first
			} else if report.Source != "" {
				seen[report.Source] = i + 1
			}
			if e != nil {
				if errors.Is(e, blind.ErrImage) {
					row["error_code"] = "blind_image"
					report.Status = "error"
					report.Error = "blind_image"
					row["report"] = report
					partial = true
				} else {
					c.Set("audit_outcome", "failed")
					c.Set("audit_code", "trace_verify")
					write(gin.H{"type": "fatal", "code": "trace_service_error"})
					return
				}
			}
			if !write(row) {
				return
			}
		}
		outcome := "completed"
		if partial {
			outcome = "partial"
		}
		if write(gin.H{"type": "end", "status": outcome}) {
			c.Set("audit_outcome", outcome)
		}
	})
}

func validTraceFiles(files []*multipart.FileHeader) bool {
	if len(files) < 1 || len(files) > 20 {
		return false
	}
	var total int64
	for _, f := range files {
		if f == nil || f.Size < 0 || f.Size > 50<<20 || len(f.Filename) > 255 {
			return false
		}
		total += f.Size
	}
	return total <= 128<<20
}
