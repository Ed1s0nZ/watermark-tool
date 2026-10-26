package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"watermark-tool/internal/blind"
)

var blindSlots = make(chan struct{}, 2)

// selectedAlgorithm preserves the old API default and rejects unrecognized modes.
func selectedAlgorithm(c *gin.Context, tempDir string, add bool) bool {
	algorithm := c.PostForm("algorithm")
	if algorithm == "" || algorithm == "legacy" {
		return false
	}
	if algorithm != blind.Algorithm {
		c.JSON(400, gin.H{"code": "blind_algorithm", "error": "Unknown watermark algorithm"})
		return true
	}
	handleBlind(c, tempDir, add)
	return true
}
func blindError(c *gin.Context, err error) {
	code := "requestError"
	status := http.StatusInternalServerError
	for _, known := range []error{blind.ErrNotFound, blind.ErrInvalid, blind.ErrCapacity, blind.ErrImage, blind.ErrText, blind.ErrKey} {
		if errors.Is(err, known) {
			code = known.Error()
			status = http.StatusUnprocessableEntity
			break
		}
	}
	if errors.Is(err, blind.ErrKey) {
		status = 503
	}
	c.JSON(status, gin.H{"code": code, "error": code})
}
func handleBlind(c *gin.Context, tempDir string, add bool) {
	select {
	case blindSlots <- struct{}{}:
		defer func() { <-blindSlots }()
	default:
		c.JSON(429, gin.H{"code": "blind_busy"})
		return
	}
	key, err := blind.DecodeKey(os.Getenv("WATERMARK_BLIND_KEY"))
	if err != nil {
		blindError(c, err)
		return
	}
	file, err := c.FormFile("file")
	if err != nil || file.Size > 50<<20 {
		c.JSON(400, gin.H{"code": "blind_image"})
		return
	}
	input := filepath.Join(tempDir, generateUniqueFilename(file.Filename))
	defer safeRemoveFile(input)
	if err := c.SaveUploadedFile(file, input); err != nil {
		c.JSON(500, gin.H{"code": "requestError"})
		return
	}
	c.Header("X-Watermark-Algorithm", blind.Algorithm)
	if add {
		output := filepath.Join(tempDir, generateUniqueFilename("output.png"))
		defer safeRemoveFile(output)
		if err := blind.AddFile(input, output, c.PostForm("watermark"), key); err != nil {
			blindError(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=watermarked.png")
		c.File(output)
		return
	}
	result, err := blind.ExtractFile(input, key)
	if err != nil {
		blindError(c, err)
		return
	}
	response := gin.H{"watermark": result.Watermark, "algorithm": result.Algorithm}
	if c.DefaultQuery("show_timestamp", "false") == "true" {
		response["timestamp"] = result.Timestamp
	}
	c.JSON(200, response)
}
