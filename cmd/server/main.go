package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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

// LoggingMiddleware 日志中间件
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 记录请求信息
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()
		latency := time.Since(startTime)

		log.Printf("[%s] %s %s %d %s", clientIP, method, path, statusCode, latency)
	}
}

// RateLimitMiddleware 请求频率限制中间件
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

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
		if client.Count >= MaxRequestsPerMinute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "请求频率过高，请稍后再试",
			})
			c.Abort()
			return
		}

		// 增加请求计数
		client.Count++

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
	r := gin.Default()

	// 添加中间件
	r.Use(LoggingMiddleware())
	r.Use(RateLimitMiddleware())

	// 添加CORS中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 设置上传文件大小限制
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	// 设置静态文件
	r.Static("/static", "./web/static")

	// 设置HTML模板
	r.LoadHTMLGlob("web/templates/*")

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// API路由
	api := r.Group("/api")
	{
		// 添加水印API
		api.POST("/add-watermark", func(c *gin.Context) {
			// 获取上传的文件
			file, err := c.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
				return
			}

			// 检查文件类型
			if err := watermarkService.ValidateMimeType(file.Filename); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型，请上传PDF、DOCX、XLSX、PPTX、ODT、RTF、JPG或PNG文件"})
				return
			}

			// 检查文件大小
			if file.Size > service.MaxFileSize {
				c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小超过限制"})
				return
			}

			// 获取水印文本
			watermarkText := c.PostForm("watermark")
			if strings.TrimSpace(watermarkText) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请输入水印文本"})
				return
			}

			// 检查水印文本长度
			if len(watermarkText) > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "水印文本过长，请控制在100个字符以内"})
				return
			}

			// 生成唯一的文件名前缀
			inputFilename := generateUniqueFilename(file.Filename)
			outputFilename := "watermarked_" + inputFilename

			// 保存上传的文件
			inputPath := filepath.Join(tempDir, inputFilename)
			if err := c.SaveUploadedFile(file, inputPath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败"})
				return
			}
			defer safeRemoveFile(inputPath) // 确保处理完后删除临时文件

			// 生成输出文件路径
			outputPath := filepath.Join(tempDir, outputFilename)
			defer safeRemoveFile(outputPath) // 确保处理完后删除临时文件

			// 设置处理超时
			processDone := make(chan error, 1)
			go func() {
				// 添加水印
				err := watermarkService.AddWatermark(inputPath, outputPath, watermarkText)
				processDone <- err
			}()

			// 等待处理完成或超时
			select {
			case err := <-processDone:
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("添加水印失败: %v", err)})
					return
				}
			case <-time.After(30 * time.Second):
				c.JSON(http.StatusRequestTimeout, gin.H{"error": "处理超时，请尝试使用更小的文件"})
				return
			}

			// 设置Content-Disposition头以使浏览器下载文件
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", outputFilename))
			c.Header("Content-Description", "File Transfer")
			c.Header("Content-Transfer-Encoding", "binary")
			c.Header("Cache-Control", "no-cache")

			// 返回带水印的文件
			c.File(outputPath)
		})

		// 提取水印API
		api.POST("/extract-watermark", func(c *gin.Context) {
			// 获取上传的文件
			file, err := c.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
				return
			}

			// 检查文件类型
			if err := watermarkService.ValidateMimeType(file.Filename); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型，请上传PDF、DOCX、XLSX、PPTX、ODT、RTF、JPG或PNG文件"})
				return
			}

			// 检查文件大小
			if file.Size > service.MaxFileSize {
				c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小超过限制"})
				return
			}

			// 生成唯一的文件名
			inputFilename := generateUniqueFilename(file.Filename)
			inputPath := filepath.Join(tempDir, inputFilename)

			// 保存上传的文件
			if err := c.SaveUploadedFile(file, inputPath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败"})
				return
			}
			defer safeRemoveFile(inputPath) // 确保处理完后删除临时文件

			// 设置处理超时
			var watermarkText string
			processDone := make(chan error, 1)
			go func() {
				// 提取水印
				var err error
				watermarkText, err = watermarkService.ExtractWatermark(inputPath)
				processDone <- err
			}()

			// 等待处理完成或超时
			select {
			case err := <-processDone:
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("提取水印失败: %v", err)})
					return
				}
			case <-time.After(30 * time.Second):
				c.JSON(http.StatusRequestTimeout, gin.H{"error": "处理超时，请尝试使用更小的文件"})
				return
			}

			// 返回提取的水印文本
			c.JSON(http.StatusOK, gin.H{"watermark": watermarkText})
		})

		// 获取支持的文件类型
		api.GET("/supported-types", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"types": watermarkService.GetSupportedTypes()})
		})
	}

	// 启动定期清理过期临时文件的任务
	go cleanupTempFiles(tempDir)

	// 启动另一个协程定期清理客户端请求记录
	go cleanupClientRequests()

	// 启动服务器
	log.Println("服务器已启动，访问 http://localhost:8080")
	r.Run(":8080")
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
		for ip, client := range clientRequestsMap {
			// 如果超过1小时没有请求，删除记录
			if now.Sub(client.LastReset) > time.Hour {
				delete(clientRequestsMap, ip)
			}
		}
	}
}
