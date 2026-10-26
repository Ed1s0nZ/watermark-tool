package main

import (
	"errors"
	"image"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
)

var blindSlots = make(chan struct{}, 2)

// selectedAlgorithm preserves the old API default and rejects unrecognized modes.
func selectedAlgorithm(c *gin.Context, tempDir string, add bool, stores ...*distribution.Store) bool {
	algorithm := c.PostForm("algorithm")
	if algorithm == "" || algorithm == "legacy" {
		return false
	}
	if algorithm != blind.Algorithm {
		c.JSON(400, gin.H{"code": "blind_algorithm", "error": "Unknown watermark algorithm"})
		return true
	}
	handleBlind(c, tempDir, add, stores...)
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
func handleBlind(c *gin.Context, tempDir string, add bool, stores ...*distribution.Store) {
	select {
	case blindSlots <- struct{}{}:
		defer func() { <-blindSlots }()
	default:
		c.JSON(429, gin.H{"code": "blind_busy"})
		return
	}
	var managed *distribution.Store
	if len(stores) > 0 {
		managed = stores[0]
	}
	var key []byte
	var err error
	var keyID string
	if managed != nil {
		keyID, key, err = managed.Current()
	} else {
		key, err = blind.DecodeKey(os.Getenv("WATERMARK_BLIND_KEY"))
	}
	if err != nil {
		platformError(c, err)
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
	var result blind.Result
	if managed != nil {
		var img image.Image
		img, err = blind.Load(input)
		if err == nil {
			result, keyID, err = managed.Verify(img, c.PostForm("key_id"))
		}
	} else {
		result, err = blind.ExtractFile(input, key)
	}
	if err != nil {
		platformError(c, err)
		return
	}
	response := gin.H{"watermark": result.Watermark, "algorithm": result.Algorithm}
	if managed != nil {
		record, lookupErr := managed.Lookup(result, keyID)
		if lookupErr != nil {
			platformError(c, lookupErr)
			return
		}
		response["key_id"] = keyID
		response["match"] = "unregistered"
		if record != nil {
			response["match"] = "registered"
			response["record"] = record
		}
	}
	if c.DefaultQuery("show_timestamp", "false") == "true" {
		response["timestamp"] = result.Timestamp
	}
	c.JSON(200, response)
}
