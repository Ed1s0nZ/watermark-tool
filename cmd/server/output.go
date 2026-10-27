package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"watermark-tool/internal/output"
	"watermark-tool/internal/quality"
)

func formOutput(c *gin.Context) (*output.Options, error) {
	raw := c.PostForm("output")
	if raw == "" {
		return nil, nil
	}
	var options output.Options
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.DisallowUnknownFields()
	if len(raw) > 256 || d.Decode(&options) != nil || d.Decode(new(any)) != io.EOF {
		return nil, output.ErrOptions
	}
	return output.Normalize(&options)
}
func formQuality(c *gin.Context) (quality.Policy, error) {
	var p quality.Policy
	raw := c.PostForm("quality_policy")
	if raw == "" {
		return p, nil
	}
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.DisallowUnknownFields()
	if len(raw) > 256 || d.Decode(&p) != nil || d.Decode(new(any)) != io.EOF || !p.Valid() {
		return p, quality.ErrPolicy
	}
	return p, nil
}
func outputError(c *gin.Context, err error, q quality.Summary) {
	if q.OutputSHA256 == "" {
		platformError(c, err)
		return
	}
	code := "output_verification"
	if err == quality.ErrThreshold {
		code = "quality_failed"
	}
	c.Set("audit_code", code)
	c.JSON(422, gin.H{"code": code, "quality": q})
}
func sendOutput(c *gin.Context, data []byte, q quality.Summary, o *output.Options, name string) {
	header, _ := json.Marshal(q)
	c.Header("X-Watermark-Quality", string(header))
	c.Header("Content-Disposition", "attachment; filename="+name+"."+output.Extension(o))
	c.Data(200, output.MIME(o), data)
}
