package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
	"watermark-tool/internal/delivery"
	"watermark-tool/internal/distribution"
)

func auditAction(c *gin.Context) (string, bool) {
	switch c.Request.Method + " " + c.FullPath() {
	case "POST /api/auth/login":
		return "auth.login", false
	case "POST /api/auth/logout":
		return "auth.logout", false
	case "POST /api/platform/keys":
		return "key.create", true
	case "POST /api/platform/keys/:id/state":
		return "key.state", true
	case "POST /api/platform/batches":
		return "batch.create", true
	case "POST /api/platform/batches/:id/download":
		return "batch.retry", true
	case "POST /api/platform/trace":
		return "watermark.trace", false
	case "POST /api/add-watermark":
		return "watermark.add", false
	case "POST /api/extract-watermark":
		return "watermark.extract", false
	case "POST /api/workbench/trace-batch":
		return "trace.batch", false
	case "POST /api/workbench/verify":
		return "report.verify", false
	case "POST /api/workbench/dual":
		return "watermark.dual", false
	case "POST /api/workbench/lab":
		return "experiment.run", false
	case "POST /api/jobs":
		return "task.create", true
	case "POST /api/jobs/:id/inputs":
		return "task.input", true
	case "POST /api/jobs/:id/start":
		return "task.start", true
	case "POST /api/jobs/:id/retry":
		return "task.retry", true
	case "POST /api/jobs/:id/cancel":
		return "task.cancel", true
	case "GET /api/jobs/:id/archive":
		return "task.download", false
	case "POST /api/audit/export":
		return "audit.export", false
	}
	return "", false
}
func auditMiddleware(store *distribution.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.Next()
			return
		}
		c.Set("audit_enabled", true)
		if store.AuditDegraded() {
			c.Header("X-Audit-Degraded", "true")
		}
		c.Next()
		action, domain := auditAction(c)
		if action == "" {
			return
		}
		status := c.Writer.Status()
		if domain && status < 400 {
			return
		}
		actor := c.GetString("username")
		if actor == "" {
			actor = "anonymous"
		}
		target := c.GetString("audit_target")
		if target == "" {
			value := c.Param("id")
			if len(value) == 24 || len(value) == 32 || value == "legacy-env" {
				target = value
			}
		}
		outcome := "completed"
		code := c.GetString("audit_code")
		if status >= 400 {
			outcome = "failed"
			code = fmt.Sprintf("http_%d", status)
			if known := c.GetString("audit_code"); known != "" {
				code = known
			}
		} else if value := c.GetString("audit_outcome"); value != "" {
			outcome = value
		}
		if expected, ok := c.Get("audit_bytes"); ok && status < 400 {
			if int64(c.Writer.Size()) < expected.(int64) {
				outcome = "response_interrupted"
			} else {
				outcome = "response_sent"
			}
		}
		if err := store.Audit(actor, action, target, outcome, code); err != nil {
			log.Printf("[AUDIT_DEGRADED] action=%s status=%d; audit unavailable, critical writes restricted", action, status)
		}
	}
}
func registerAudit(api *gin.RouterGroup, store *distribution.Store, managers ...*delivery.Manager) {
	api.GET("/audit/status", func(c *gin.Context) {
		taskDegraded := len(managers) > 0 && managers[0] != nil && managers[0].Degraded()
		c.JSON(200, gin.H{"task_degraded": taskDegraded, "configured": store != nil, "degraded": store != nil && store.AuditDegraded(), "max_events": 100000, "max_jobs": distribution.MaxJobs})
	})
	group := api.Group("/audit", func(c *gin.Context) {
		if store == nil {
			c.AbortWithStatusJSON(503, gin.H{"code": "platform_config"})
			return
		}
		c.Next()
	})
	for _, name := range []string{"search", "export"} {
		name := name
		group.POST("/"+name, func(c *gin.Context) {
			filter := distribution.AuditFilter{Page: 1, Size: 20}
			if !smallJSON(c, &filter) {
				return
			}
			rows, total, err := store.AuditSearch(filter, name == "export")
			if err != nil {
				platformError(c, err)
				return
			}
			if name == "search" {
				c.JSON(200, gin.H{"items": rows, "total": total, "degraded": store.AuditDegraded()})
				return
			}
			if filter.Format == "json" || filter.Format == "" {
				c.Header("Content-Disposition", "attachment; filename=audit.json")
				c.JSON(200, gin.H{"items": rows, "total": total})
				return
			}
			if filter.Format != "csv" {
				platformError(c, distribution.ErrRecipients)
				return
			}
			var buffer bytes.Buffer
			out := csv.NewWriter(&buffer)
			_ = out.Write([]string{"sequence", "id", "at", "actor", "action", "target", "correlation", "outcome", "code"})
			for _, row := range rows {
				_ = out.Write([]string{strconv.FormatUint(row.Sequence, 10), row.ID, row.At.Format(time.RFC3339Nano), distribution.CSVCell(row.Actor), row.Action, row.Target, row.Correlation, row.Outcome, row.Code})
			}
			out.Flush()
			if out.Error() != nil {
				platformError(c, out.Error())
				return
			}
			c.Header("Content-Disposition", "attachment; filename=audit.csv")
			c.Data(http.StatusOK, "text/csv; charset=utf-8", buffer.Bytes())
		})
	}
}

func setAuditFile(c *gin.Context, path string) {
	if !c.GetBool("audit_enabled") {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err == nil {
		c.Set("audit_target", hex.EncodeToString(h.Sum(nil)))
	}
}
