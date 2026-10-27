package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/quality"
	"watermark-tool/internal/workbench"
)

type verificationReport struct {
	Kind         string               `json:"kind"`
	Schema       int                  `json:"schema_version"`
	Source       string               `json:"source_sha256"`
	Width        int                  `json:"width"`
	Height       int                  `json:"height"`
	TestedAt     string               `json:"tested_at"`
	Milliseconds int64                `json:"elapsed_ms"`
	Algorithm    string               `json:"algorithm"`
	Status       string               `json:"status"`
	Error        string               `json:"error_code,omitempty"`
	Result       *blind.Result        `json:"result,omitempty"`
	KeyID        string               `json:"key_id,omitempty"`
	Record       *distribution.Record `json:"record,omitempty"`
}

func workbenchKey(store *distribution.Store) (string, []byte, error) {
	if store != nil {
		return store.Current()
	}
	key, err := blind.DecodeKey(os.Getenv("WATERMARK_BLIND_KEY"))
	return "", key, err
}
func verifyWorkbench(img image.Image, store *distribution.Store, keyID string) (blind.Result, string, error) {
	if store != nil {
		return store.Verify(img, keyID)
	}
	if keyID != "" {
		return blind.Result{}, "", distribution.ErrMissing
	}
	_, key, err := workbenchKey(nil)
	if err != nil {
		return blind.Result{}, "", err
	}
	r, err := blind.Extract(img, key)
	return r, "", err
}
func registerWorkbench(api *gin.RouterGroup, store *distribution.Store, tempDir string) {
	group := api.Group("/workbench", RateLimitMiddleware(), func(c *gin.Context) {
		limit := int64(51 << 20)
		if c.Request.URL.Path == "/api/workbench/dual" {
			limit = 101 << 20
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		defer func() {
			if c.Request.MultipartForm != nil {
				_ = c.Request.MultipartForm.RemoveAll()
			}
		}()
		c.Next()
	})
	group.POST("/lab", func(c *gin.Context) {
		withPlatformImage(c, tempDir, func(input string) {
			img, err := blind.Load(input)
			if err != nil {
				platformError(c, err)
				return
			}
			id, key, err := workbenchKey(store)
			if err != nil {
				platformError(c, err)
				return
			}
			data, err := os.ReadFile(input)
			if err != nil {
				platformError(c, err)
				return
			}
			source := workbench.Digest(data)
			ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
			defer cancel()
			report, err := workbench.Run(ctx, img, c.PostForm("watermark"), key, time.Now())
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					c.JSON(http.StatusRequestTimeout, gin.H{"code": "workbench_cancelled"})
					return
				}
				platformError(c, err)
				return
			}
			after, err := os.ReadFile(input)
			if err != nil {
				platformError(c, err)
				return
			}
			report.Source = source
			report.KeyID = id
			report.OriginalIntact = source == workbench.Digest(after)
			c.JSON(200, report)
		})
	})
	group.POST("/dual", func(c *gin.Context) {
		withPlatformImage(c, tempDir, func(input string) {
			img, err := blind.Load(input)
			if err != nil {
				platformError(c, err)
				return
			}
			_, key, err := workbenchKey(store)
			if err != nil {
				platformError(c, err)
				return
			}
			var policy quality.Policy
			if raw := c.PostForm("quality_policy"); raw != "" {
				d := json.NewDecoder(bytes.NewBufferString(raw))
				d.DisallowUnknownFields()
				if len(raw) > 256 || d.Decode(&policy) != nil || d.Decode(new(any)) != io.EOF || !policy.Valid() {
					platformError(c, quality.ErrPolicy)
					return
				}
			}
			var original image.Image
			if originalFile, e := c.FormFile("original"); e == nil {
				if originalFile.Size > 50<<20 {
					platformError(c, blind.ErrImage)
					return
				}
				path := filepath.Join(filepath.Dir(input), "original")
				if e = c.SaveUploadedFile(originalFile, path); e != nil {
					platformError(c, e)
					return
				}
				original, e = blind.Load(path)
				if e != nil {
					platformError(c, e)
					return
				}
			}
			now := time.Now()
			text := c.PostForm("watermark")
			out, err := blind.Embed(img, text, key, now)
			if err != nil {
				platformError(c, err)
				return
			}
			actual, err := blind.Extract(out, key)
			if err != nil {
				platformError(c, err)
				return
			}
			if actual.Watermark != text || actual.Timestamp != now.UTC().Format(time.RFC3339) {
				platformError(c, blind.ErrInvalid)
				return
			}
			if c.Request.Context().Err() != nil {
				return
			}
			summary := quality.Compare(img, out, original, policy)
			if !summary.Accepted {
				c.Set("audit_code", "quality_failed")
				c.JSON(422, gin.H{"code": "quality_failed", "quality": summary})
				return
			}
			header, _ := json.Marshal(summary)
			c.Header("X-Watermark-Quality", string(header))
			output := filepath.Join(filepath.Dir(input), "dual.png")
			if err = blind.SaveNew(output, out); err != nil {
				platformError(c, err)
				return
			}
			c.Header("Content-Disposition", "attachment; filename=dual-watermark.png")
			c.File(output)
		})
	})
	group.POST("/verify", func(c *gin.Context) {
		withPlatformImage(c, tempDir, func(input string) {
			report, err := buildVerification(input, store, c.PostForm("key_id"))
			if err != nil {
				platformError(c, err)
				return
			}
			c.JSON(200, report)
		})
	})
}
