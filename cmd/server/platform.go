package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/composition"
	"watermark-tool/internal/delivery"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/output"
	"watermark-tool/internal/quality"
)

func openPlatform() (*distribution.Store, error) {
	dir, raw := os.Getenv("WATERMARK_DATA_DIR"), os.Getenv("WATERMARK_DATA_KEY")
	if dir == "" && raw == "" {
		return nil, nil
	}
	master, err := blind.DecodeKey(raw)
	if err != nil {
		return nil, distribution.ErrConfig
	}
	var legacy []byte
	if value := os.Getenv("WATERMARK_BLIND_KEY"); value != "" {
		legacy, err = blind.DecodeKey(value)
		if err != nil {
			return nil, distribution.ErrConfig
		}
	}
	return distribution.Open(dir, master, legacy)
}
func platformError(c *gin.Context, err error) {
	status, code := 500, "requestError"
	switch {
	case errors.Is(err, output.ErrOptions):
		status, code = 422, "output_options"
	case errors.Is(err, output.ErrVerification):
		status, code = 422, "output_verification"
	case errors.Is(err, composition.ErrArtwork):
		status, code = 422, "artwork_invalid"
	case errors.Is(err, quality.ErrPolicy):
		status, code = 422, "quality_policy"
	case errors.Is(err, quality.ErrThreshold):
		status, code = 422, "quality_failed"
	case errors.Is(err, distribution.ErrTemplate):
		status, code = 422, "job_template"
	case errors.Is(err, distribution.ErrJobState):
		status, code = 409, "job_state"
	case errors.Is(err, delivery.ErrExpired):
		status, code = 410, "archive_expired"
	case errors.Is(err, delivery.ErrArchive):
		status, code = 409, "archive_unavailable"
	case errors.Is(err, distribution.ErrStore), errors.Is(err, distribution.ErrConfig):
		status, code = 503, "store_unavailable"
	case errors.Is(err, distribution.ErrConflict):
		status, code = 409, "request_conflict"
	case errors.Is(err, distribution.ErrBusy):
		status, code = 409, "batch_busy"
	case errors.Is(err, distribution.ErrRecipients):
		status, code = 400, "invalid_recipients"
	case errors.Is(err, distribution.ErrLimit):
		status, code = 422, "platform_limit"
	case errors.Is(err, distribution.ErrMissing):
		status, code = 404, "record_not_found"
	case errors.Is(err, distribution.ErrDisabled):
		status, code = 409, "key_disabled"
	case errors.Is(err, distribution.ErrState):
		status, code = 409, "invalid_key_state"
	default:
		blindError(c, err)
		return
	}
	c.Set("audit_code", code)
	c.JSON(status, gin.H{"code": code})
}
func smallJSON(c *gin.Context, v any) bool { return boundedJSON(c, v, 16<<10) }
func boundedJSON(c *gin.Context, v any, limit int64) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		c.JSON(400, gin.H{"code": "invalid_recipients"})
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		c.JSON(400, gin.H{"code": "invalid_recipients"})
		return false
	}
	return true
}
func registerPlatform(api *gin.RouterGroup, store *distribution.Store, tempDir string) {
	group := api.Group("/platform", func(c *gin.Context) {
		if store == nil {
			c.AbortWithStatusJSON(503, gin.H{"code": "platform_config"})
			return
		}
		c.Next()
	})
	group.GET("/keys", func(c *gin.Context) {
		keys, err := store.Keys()
		if err != nil {
			platformError(c, err)
			return
		}
		c.JSON(200, gin.H{"items": keys})
	})
	group.POST("/keys", RateLimitMiddleware(), func(c *gin.Context) {
		key, err := store.CreateKey(c.GetString("username"))
		if err != nil {
			platformError(c, err)
			return
		}
		c.JSON(201, key)
	})
	group.POST("/keys/:id/state", RateLimitMiddleware(), func(c *gin.Context) {
		var request struct {
			State string `json:"state"`
		}
		if !smallJSON(c, &request) {
			return
		}
		key, err := store.SetKeyState(c.Param("id"), request.State, c.GetString("username"))
		if err != nil {
			platformError(c, err)
			return
		}
		c.JSON(200, key)
	})
	for _, kind := range []string{"batches", "records"} {
		kind := kind
		group.POST("/"+kind+"/search", func(c *gin.Context) {
			request := struct {
				Q    string `json:"q"`
				Page int    `json:"page"`
				Size int    `json:"page_size"`
			}{Page: 1, Size: 20}
			if !smallJSON(c, &request) {
				return
			}
			if !distribution.ValidPage(request.Page, request.Size) {
				platformError(c, distribution.ErrRecipients)
				return
			}
			var items any
			var total int
			var err error
			if kind == "batches" {
				items, total, err = store.Batches(request.Q, request.Page, request.Size)
			} else {
				items, total, err = store.Records(request.Q, request.Page, request.Size)
			}
			if err != nil {
				platformError(c, err)
				return
			}
			c.JSON(200, gin.H{"items": items, "total": total, "page": request.Page, "page_size": request.Size})
		})
	}
	group.POST("/batches", RateLimitMiddleware(), func(c *gin.Context) { platformBatch(c, store, tempDir, "") })
	group.POST("/batches/:id/download", RateLimitMiddleware(), func(c *gin.Context) { platformBatch(c, store, tempDir, c.Param("id")) })
	group.POST("/trace", RateLimitMiddleware(), func(c *gin.Context) {
		withPlatformImage(c, tempDir, func(input string) {
			img, err := blind.Load(input)
			if err != nil {
				platformError(c, err)
				return
			}
			result, keyID, err := store.Verify(img, c.PostForm("key_id"))
			if err != nil {
				platformError(c, err)
				return
			}
			record, err := store.Lookup(result, keyID)
			if err != nil {
				platformError(c, err)
				return
			}
			match := "unregistered"
			if record != nil {
				match = "registered"
			}
			c.JSON(200, gin.H{"watermark": result.Watermark, "timestamp": result.Timestamp, "algorithm": result.Algorithm, "key_id": keyID, "match": match, "record": record})
		})
	})
}
func withPlatformImage(c *gin.Context, tempDir string, process func(string)) {
	select {
	case blindSlots <- struct{}{}:
		defer func() { <-blindSlots }()
	default:
		c.JSON(429, gin.H{"code": "blind_busy"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil || file.Size > 50<<20 {
		platformError(c, blind.ErrImage)
		return
	}
	dir, err := os.MkdirTemp(tempDir, "distribution-")
	if err != nil {
		platformError(c, err)
		return
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input")
	if err = c.SaveUploadedFile(file, input); err != nil {
		platformError(c, err)
		return
	}
	setAuditFile(c, input)
	process(input)
}
func platformBatch(c *gin.Context, store *distribution.Store, tempDir, resumeID string) {
	withPlatformImage(c, tempDir, func(input string) {
		img, err := blind.Load(input)
		if err == nil {
			err = blind.Validate(img)
		}
		if err != nil {
			platformError(c, err)
			return
		}
		data, err := os.ReadFile(input)
		if err != nil {
			platformError(c, err)
			return
		}
		source := distribution.Digest(data)
		var batch distribution.Batch
		var key []byte
		if resumeID != "" {
			batch, key, err = store.Resume(resumeID, source)
		} else {
			var recipients []distribution.Recipient
			recipients, err = distribution.ParseRecipients(c.PostForm("recipients_csv"))
			if err == nil {
				batch, key, err = store.Begin(c.GetHeader("Idempotency-Key"), source, c.GetString("username"), recipients)
			}
		}
		if err != nil {
			platformError(c, err)
			return
		}
		finalized := false
		defer func() {
			if !finalized {
				_ = store.Finish(batch.ID, "generation_interrupted")
			}
		}()
		output := filepath.Join(filepath.Dir(input), "copies.zip")
		if err = distribution.ArchiveContext(c.Request.Context(), output, img, batch, key); err != nil {
			finishErr := store.Finish(batch.ID, "generation_failed")
			finalized = true
			if finishErr != nil {
				platformError(c, finishErr)
				return
			}
			platformError(c, err)
			return
		}
		if err = store.Finish(batch.ID, ""); err != nil {
			platformError(c, err)
			return
		}
		finalized = true
		c.Header("X-Batch-ID", batch.ID)
		c.Header("Content-Disposition", "attachment; filename="+batch.ID+".zip")
		c.File(output)
	})
}
