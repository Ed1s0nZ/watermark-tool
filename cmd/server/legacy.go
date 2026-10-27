package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/service"
)

// registerLegacy keeps the existing document/Go single-file API reusable by scoped engines.
func registerLegacy(api *gin.RouterGroup, managed *distribution.Store, watermarkService *service.WatermarkService, tempDir string) {

	{
		// 添加水印API
		api.POST("/add-watermark", RateLimitMiddleware(), func(c *gin.Context) {
			if selectedAlgorithm(c, tempDir, true, managed) {
				return
			}
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
			setAuditFile(c, inputPath)

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
		api.POST("/extract-watermark", RateLimitMiddleware(), func(c *gin.Context) {
			if selectedAlgorithm(c, tempDir, false, managed) {
				return
			}
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

			// 是否显示时间戳
			showTimestamp := c.DefaultQuery("show_timestamp", "false") == "true"

			// 生成唯一的文件名
			inputFilename := generateUniqueFilename(file.Filename)
			inputPath := filepath.Join(tempDir, inputFilename)

			// 保存上传的文件
			if err := c.SaveUploadedFile(file, inputPath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败"})
				return
			}
			defer safeRemoveFile(inputPath) // 确保处理完后删除临时文件
			setAuditFile(c, inputPath)

			// 设置处理超时
			var watermarkText string
			var timestamp string
			processDone := make(chan error, 1)

			if showTimestamp {
				go func() {
					// 提取水印和时间戳
					var err error
					watermarkText, timestamp, err = watermarkService.ExtractWatermarkWithTimestamp(inputPath)
					processDone <- err
				}()
			} else {
				go func() {
					// 只提取水印
					var err error
					watermarkText, err = watermarkService.ExtractWatermark(inputPath)
					processDone <- err
				}()
			}

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
			if showTimestamp {
				// 格式化时间戳
				formattedTimestamp := formatTimestamp(timestamp)
				c.JSON(http.StatusOK, gin.H{
					"watermark": watermarkText,
					"timestamp": formattedTimestamp,
				})
			} else {
				c.JSON(http.StatusOK, gin.H{"watermark": watermarkText})
			}
		})

		// 获取支持的文件类型
		api.GET("/supported-types", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"types": watermarkService.GetSupportedTypes()})
		})
	}

}
