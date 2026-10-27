package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
	"watermark-tool/internal/delivery"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/quality"
	"watermark-tool/internal/service"
)

func registerJobs(api *gin.RouterGroup, store *distribution.Store, manager *delivery.Manager, tempDir string) {
	var uploadMu sync.Mutex
	var uploads int
	window := time.Now()
	group := api.Group("/jobs", func(c *gin.Context) {
		if store == nil || manager == nil {
			c.AbortWithStatusJSON(503, gin.H{"code": "platform_config"})
			return
		}
		c.Next()
	})
	// Each input remains a bounded single-image request; total source/rendered budgets are enforced per task.
	group.Use(func(c *gin.Context) {
		if c.Request.Method == "POST" {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxFileSize+(1<<20))
			defer func() {
				if c.Request.MultipartForm != nil {
					_ = c.Request.MultipartForm.RemoveAll()
				}
			}()
		}
		c.Next()
	})
	group.POST("/preview-plan", func(c *gin.Context) {
		var request struct {
			Quality *quality.Policy `json:"quality_policy"`
			CSV     string          `json:"recipients_csv"`
			Mode    string          `json:"mode"`
			Text    string          `json:"template_text"`
			Style   map[string]any  `json:"style"`
		}
		if !boundedJSON(c, &request, 32<<10) {
			return
		}
		if request.Quality != nil && !request.Quality.Valid() {
			platformError(c, quality.ErrPolicy)
			return
		}
		recipients, err := distribution.ParseRecipients(request.CSV)
		if err != nil {
			platformError(c, err)
			return
		}
		if request.Mode != "blind" && request.Mode != "dual" {
			platformError(c, distribution.ErrRecipients)
			return
		}
		if request.Mode == "dual" {
			if !distribution.ValidStyle(request.Style) {
				platformError(c, distribution.ErrTemplate)
				return
			}
			for _, r := range recipients {
				if _, err = distribution.VisibleText(request.Text, r); err != nil {
					platformError(c, err)
					return
				}
			}
		}
		c.JSON(200, gin.H{"recipients": recipients})
	})

	group.POST("", RateLimitMiddleware(), func(c *gin.Context) {
		if manager.Degraded() {
			platformError(c, distribution.ErrStore)
			return
		}
		var request struct {
			Quality *quality.Policy          `json:"quality_policy"`
			Sources []distribution.JobSource `json:"sources"`
			CSV     string                   `json:"recipients_csv"`
			Mode    string                   `json:"mode"`
			Text    string                   `json:"template_text"`
			Style   map[string]any           `json:"style"`
		}
		if !boundedJSON(c, &request, 32<<10) {
			return
		}
		if request.Quality != nil && !request.Quality.Valid() {
			platformError(c, quality.ErrPolicy)
			return
		}
		recipients, err := distribution.ParseRecipients(request.CSV)
		if err != nil {
			platformError(c, err)
			return
		}
		policies := []quality.Policy{}
		if request.Quality != nil {
			policies = append(policies, *request.Quality)
		}
		job, err := store.NewJob(c.GetHeader("Idempotency-Key"), c.GetString("username"), request.Sources, recipients, request.Mode, request.Text, request.Style, policies...)
		if err != nil {
			platformError(c, err)
			return
		}
		c.JSON(201, job)
	})
	group.POST("/search", func(c *gin.Context) {
		request := struct {
			Q    string `json:"q"`
			Page int    `json:"page"`
			Size int    `json:"page_size"`
		}{Page: 1, Size: 20}
		if !smallJSON(c, &request) {
			return
		}
		rows, total, err := store.Jobs(request.Q, request.Page, request.Size)
		if err != nil {
			platformError(c, err)
			return
		}
		for i, j := range rows {
			if view, e := manager.View(j.ID); e == nil {
				rows[i] = view
			}
		}
		c.JSON(200, gin.H{"items": rows, "total": total})
	})
	group.GET("/:id", func(c *gin.Context) {
		job, err := manager.View(c.Param("id"))
		if err != nil {
			platformError(c, err)
			return
		}
		c.JSON(200, job)
	})
	group.POST("/:id/inputs", func(c *gin.Context) {
		uploadMu.Lock()
		if time.Since(window) >= time.Minute {
			window = time.Now()
			uploads = 0
		}
		if uploads >= 240 {
			uploadMu.Unlock()
			c.Header("Retry-After", "60")
			c.JSON(429, gin.H{"code": "tooMany"})
			return
		}
		uploads++
		uploadMu.Unlock()
		withPlatformImage(c, tempDir, func(input string) {
			token := c.GetHeader("X-Upload-Token")
			if len(token) != 32 {
				platformError(c, distribution.ErrConflict)
				return
			}
			job, err := manager.Upload(c.Param("id"), c.PostForm("source_id"), c.PostForm("copy_id"), input, token)
			if err != nil {
				platformError(c, err)
				return
			}
			c.JSON(200, job)
		})
	})
	for _, name := range []string{"start", "retry", "cancel"} {
		name := name
		group.POST("/:id/"+name, RateLimitMiddleware(), func(c *gin.Context) {
			var job distribution.Job
			var err error
			if name == "cancel" {
				job, err = manager.Cancel(c.Param("id"), c.GetString("username"))
			} else {
				job, err = manager.Start(c.Param("id"), c.GetString("username"), name == "retry")
			}
			if err != nil {
				platformError(c, err)
				return
			}
			c.JSON(200, job)
		})
	}
	group.GET("/:id/archive", RateLimitMiddleware(), func(c *gin.Context) {
		id := c.Param("id")
		if err := store.Audit(c.GetString("username"), "task.download", id, "attempt", ""); err != nil {
			platformError(c, err)
			return
		}
		f, job, err := manager.OpenArchive(id)
		if err != nil {
			platformError(c, err)
			return
		}
		defer f.Close()
		c.Set("audit_bytes", job.ArchiveBytes)
		c.DataFromReader(200, job.ArchiveBytes, "application/zip", f, map[string]string{"Content-Disposition": "attachment; filename=" + job.ID + ".zip"})
	})
}

func deliveryOptions() (delivery.Options, error) {
	options := delivery.Options{}
	if raw := os.Getenv("WATERMARK_JOB_ARCHIVE_TTL_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 1 || seconds > 3600 {
			return options, distribution.ErrConfig
		}
		options.ArchiveTTL = time.Duration(seconds) * time.Second
	}
	return options, nil
}
