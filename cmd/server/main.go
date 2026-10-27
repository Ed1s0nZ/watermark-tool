package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"watermark-tool/internal/auth"
	"watermark-tool/internal/delivery"
	"watermark-tool/internal/service"
	_ "watermark-tool/internal/watermark/docx"
	_ "watermark-tool/internal/watermark/jpg"
	_ "watermark-tool/internal/watermark/odt"
	_ "watermark-tool/internal/watermark/pdf"
	_ "watermark-tool/internal/watermark/png"
	_ "watermark-tool/internal/watermark/pptx"
	_ "watermark-tool/internal/watermark/rtf"
	_ "watermark-tool/internal/watermark/xlsx"
)

// 请求频率限制（每分钟最多10次请求）
const (
	MaxRequestsPerMinute = 10
	CleanupInterval      = 1 * time.Hour
)

// ClientRequests 存储客户端请求频率
type ClientRequests struct {
	Count     int
	LastReset time.Time
}

// 存储客户端请求频率的映射
var clientRequestsMap = make(map[string]*ClientRequests)
var clientRequestsMu sync.Mutex

// LoggingMiddleware 日志中间件
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 记录请求信息
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = "<unmatched>"
		}
		statusCode := c.Writer.Status()
		latency := time.Since(startTime)

		log.Printf("[%s] %s %s %d %s", clientIP, method, path, statusCode, latency)
	}
}

// RateLimitMiddleware 请求频率限制中间件
func RateLimitMiddleware() gin.HandlerFunc { return rateLimitBucket("", MaxRequestsPerMinute) }
func rateLimitBucket(prefix string, limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := prefix + c.ClientIP()

		clientRequestsMu.Lock()
		// 获取或创建客户端请求记录
		client, exists := clientRequestsMap[clientIP]
		if !exists {
			client = &ClientRequests{
				Count:     0,
				LastReset: time.Now(),
			}
			clientRequestsMap[clientIP] = client
		}

		// 检查是否需要重置计数器
		if time.Since(client.LastReset) > time.Minute {
			client.Count = 0
			client.LastReset = time.Now()
		}

		// 检查请求频率
		if client.Count >= limit {
			remaining := int(time.Until(client.LastReset.Add(time.Minute)).Seconds()) + 1
			if remaining < 1 {
				remaining = 1
			}
			c.Header("Retry-After", strconv.Itoa(remaining))
			clientRequestsMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "请求频率过高，请稍后再试",
			})
			c.Abort()
			return
		}

		// 增加请求计数
		client.Count++
		clientRequestsMu.Unlock()

		c.Next()
	}
}

// generateUniqueFilename 生成唯一的文件名
func generateUniqueFilename(originalFilename string) string {
	// 获取文件扩展名
	ext := filepath.Ext(originalFilename)
	// 生成UUID作为文件名
	return uuid.New().String() + ext
}

// 安全地移除临时文件
func safeRemoveFile(filePath string) {
	if _, err := os.Stat(filePath); err == nil {
		os.Remove(filePath)
	}
}

func main() {
	exitCode := 0
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()
	// 创建水印服务
	watermarkService := service.NewWatermarkService()

	// 创建临时目录用于存储上传和处理的文件
	tempDir, err := os.MkdirTemp("", "watermark-uploads-*")
	if err != nil {
		log.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 设置为发布模式
	gin.SetMode(gin.ReleaseMode)

	// 创建Gin实例
	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(LoggingMiddleware())
	managed, err := openPlatform()
	if err != nil {
		log.Fatalf("Persistent platform configuration/state unavailable: %v", err)
	}
	if managed != nil {
		defer managed.Close()
		os.RemoveAll(tempDir)
		tempDir = managed.RuntimeDir()
		defer os.RemoveAll(tempDir)
	}
	var tasks *delivery.Manager
	if managed != nil {
		options, e := deliveryOptions()
		if e != nil {
			log.Fatalf("Invalid task retention configuration: %v", e)
		}
		tasks, err = delivery.New(managed, blindSlots, options)
		if err != nil {
			log.Fatalf("Task storage unavailable: %v", err)
		}
		defer tasks.Close()
	}
	r.SetTrustedProxies(nil)
	sessions := auth.New(os.Getenv("WATERMARK_ADMIN_USER"), os.Getenv("WATERMARK_ADMIN_PASSWORD"), os.Getenv("WATERMARK_COOKIE_SECURE") == "true")
	collaboration, err := openWorkspace(managed)
	if err != nil {
		log.Fatalf("Collaboration metadata unavailable: %v", err)
	}
	var engines *teamEngines
	if collaboration != nil {
		defer collaboration.Close()
		sessions.UseIdentityProvider(auth.IdentityProvider{Authenticate: collaboration.Authenticate, Version: collaboration.CredentialVersion})
		engines, err = newTeamEngines(collaboration, managed, tasks)
		if err != nil {
			log.Fatalf("Team engine configuration unavailable: %v", err)
		}
		defer engines.Close()
	}

	// 设置上传文件大小限制
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	if _, err := os.Stat("web/dist/index.html"); err != nil {
		log.Fatal("Frontend missing: run cd web/frontend && npm ci && npm run build")
	}
	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/favicon.svg", "./web/dist/favicon.svg")
	r.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.File("web/dist/index.html")
	})
	authAPI := r.Group("/api/auth", auth.SameOrigin(), auditMiddleware(managed))
	authAPI.GET("/session", sessions.Session)
	authAPI.POST("/login", RateLimitMiddleware(), sessions.Login)
	authAPI.POST("/logout", sessions.Logout)
	api := r.Group("/api", auth.SameOrigin(), sessions.Require(), auditMiddleware(managed))
	registerWorkspace(api, collaboration, engines)
	api.Use(requireEnvironmentAdmin)
	registerJobs(api, managed, tasks, tempDir)
	registerAudit(api, managed, tasks)
	registerBatchTrace(api, managed, tempDir)
	registerWorkbench(api, managed, tempDir)
	api.Use(func(c *gin.Context) {
		if c.Request.Method == http.MethodPost {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxFileSize+(1<<20))
			defer func() {
				if c.Request.MultipartForm != nil {
					c.Request.MultipartForm.RemoveAll()
				}
			}()
		}
		c.Next()
	})

	registerPlatform(api, managed, tempDir)
	registerLegacy(api, managed, watermarkService, tempDir)

	// 启动定期清理过期临时文件的任务
	go cleanupTempFiles(tempDir)

	// 启动另一个协程定期清理客户端请求记录
	go cleanupClientRequests()

	// 启动服务器
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("服务器已启动，访问 http://localhost:%s", port)
	server := &http.Server{Addr: ":" + port, Handler: r, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stopped := make(chan error, 1)
	go func() { stopped <- server.ListenAndServe() }()
	select {
	case err := <-stopped:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Server stopped: %v", err)
			exitCode = 1
		}
	case <-stopContext.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			server.Close()
		}
	}

}

// cleanupTempFiles 定期清理临时文件
func cleanupTempFiles(tempDir string) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 跳过目录
			if info.IsDir() {
				return nil
			}

			// 如果文件超过1小时，删除
			if now.Sub(info.ModTime()) > time.Hour {
				if err := os.Remove(path); err != nil {
					log.Printf("删除文件失败 %s: %v", path, err)
				}
			}

			return nil
		})

		if err != nil {
			log.Printf("清理临时文件时出错: %v", err)
		}
	}
}

// cleanupClientRequests 定期清理客户端请求记录
func cleanupClientRequests() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		clientRequestsMu.Lock()
		for ip, client := range clientRequestsMap {
			// 如果超过1小时没有请求，删除记录
			if now.Sub(client.LastReset) > time.Hour {
				delete(clientRequestsMap, ip)
			}
		}
		clientRequestsMu.Unlock()
	}
}

// formatTimestamp 格式化时间戳
func formatTimestamp(timestamp string) string {
	// 尝试解析为 Unix 时间戳（秒）
	if unixTime, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		return time.Unix(unixTime, 0).Format("2006-01-02 15:04:05")
	}

	// 尝试解析为 RFC3339 格式
	if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
		return t.Format("2006-01-02 15:04:05")
	}

	// 无法解析，直接返回原始字符串
	return timestamp
}
