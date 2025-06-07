package docx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"watermark-tool/internal/watermark"
)

// DOCXWatermarker 实现Word文档的水印处理
type DOCXWatermarker struct{}

// 注册Word水印处理器
func init() {
	watermark.RegisterWatermarker(&DOCXWatermarker{})
}

// AddWatermark 为Word文档添加水印
// 注意：由于文档格式的复杂性，这里使用一个简化的实现
// 实际应用中可能需要更复杂的方法
func (d *DOCXWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "docx-watermark-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压DOCX文件
	err = unzipFile(inputFile, tempDir)
	if err != nil {
		return fmt.Errorf("解压DOCX文件失败: %w", err)
	}

	// 向文档添加水印
	// 1. 修改document.xml，添加水印到页眉
	documentPath := filepath.Join(tempDir, "word", "document.xml")
	docContent, err := os.ReadFile(documentPath)
	if err != nil {
		return fmt.Errorf("读取document.xml失败: %w", err)
	}

	// 存储水印信息到文档属性
	// 查找文档属性位置
	corePropsPath := filepath.Join(tempDir, "docProps", "core.xml")
	if _, err := os.Stat(corePropsPath); err == nil {
		coreContent, err := os.ReadFile(corePropsPath)
		if err == nil {
			// 在关键词部分添加水印信息
			modified := false
			if bytes.Contains(coreContent, []byte("<cp:keywords>")) {
				newContent := bytes.Replace(
					coreContent,
					[]byte("<cp:keywords>"),
					[]byte(fmt.Sprintf("<cp:keywords>Watermark:%s ", watermarkText)),
					1,
				)
				if !bytes.Equal(newContent, coreContent) {
					coreContent = newContent
					modified = true
				}
			} else if bytes.Contains(coreContent, []byte("</cp:coreProperties>")) {
				// 如果没有关键词标签，添加一个
				newContent := bytes.Replace(
					coreContent,
					[]byte("</cp:coreProperties>"),
					[]byte(fmt.Sprintf("<cp:keywords>Watermark:%s</cp:keywords></cp:coreProperties>", watermarkText)),
					1,
				)
				if !bytes.Equal(newContent, coreContent) {
					coreContent = newContent
					modified = true
				}
			}

			if modified {
				err = os.WriteFile(corePropsPath, coreContent, 0644)
				if err != nil {
					return fmt.Errorf("写入core.xml失败: %w", err)
				}
			}
		}
	}

	// 添加简单的水印标记到文档内容
	// 注意：这是一个简化版本，只是在文档中添加一个不可见的标记
	watermarkTag := fmt.Sprintf("<!-- Watermark: %s -->", watermarkText)
	if bytes.Contains(docContent, []byte("<w:body>")) {
		docContent = bytes.Replace(
			docContent,
			[]byte("<w:body>"),
			[]byte(fmt.Sprintf("<w:body>%s", watermarkTag)),
			1,
		)
		err = os.WriteFile(documentPath, docContent, 0644)
		if err != nil {
			return fmt.Errorf("写入document.xml失败: %w", err)
		}
	}

	// 重新打包DOCX文件
	err = zipDir(tempDir, outputFile)
	if err != nil {
		return fmt.Errorf("重新打包DOCX文件失败: %w", err)
	}

	return nil
}

// ExtractWatermark 从DOCX文档中提取水印
func (d *DOCXWatermarker) ExtractWatermark(inputFile string) (string, string, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "docx-extract-*")
	if err != nil {
		return "", "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压DOCX文件
	err = unzipFile(inputFile, tempDir)
	if err != nil {
		return "", "", fmt.Errorf("解压DOCX文件失败: %w", err)
	}

	// 默认时间戳
	timestamp := time.Now().Format(time.RFC3339)

	// 首先检查文档属性
	corePropsPath := filepath.Join(tempDir, "docProps", "core.xml")
	if _, err := os.Stat(corePropsPath); err == nil {
		coreContent, err := os.ReadFile(corePropsPath)
		if err == nil {
			// 查找时间戳信息
			timeStampPrefix := "TimeStamp:"
			if tsIdx := bytes.Index(coreContent, []byte(timeStampPrefix)); tsIdx > 0 {
				tsStart := tsIdx + len(timeStampPrefix)
				tsEnd := tsStart
				for i := tsStart; i < len(coreContent) && i < tsStart+50; i++ {
					if coreContent[i] == '<' || coreContent[i] == ' ' {
						tsEnd = i
						break
					}
				}
				if tsEnd > tsStart {
					timestamp = string(coreContent[tsStart:tsEnd])
				}
			}

			// 查找水印信息
			watermarkPrefix := "Watermark:"
			if idx := bytes.Index(coreContent, []byte(watermarkPrefix)); idx > 0 {
				start := idx + len(watermarkPrefix)
				end := start
				for i := start; i < len(coreContent) && i < start+100; i++ {
					if coreContent[i] == '<' || coreContent[i] == ' ' {
						end = i
						break
					}
				}
				if end > start {
					return string(coreContent[start:end]), timestamp, nil
				}
			}
		}
	}

	// 如果在文档属性中没找到，查找文档内容
	documentPath := filepath.Join(tempDir, "word", "document.xml")
	docContent, err := os.ReadFile(documentPath)
	if err != nil {
		return "", "", fmt.Errorf("读取document.xml失败: %w", err)
	}

	// 查找时间戳标记
	timeStampTagPrefix := "<!-- TimeStamp: "
	if tsIdx := bytes.Index(docContent, []byte(timeStampTagPrefix)); tsIdx > 0 {
		tsStart := tsIdx + len(timeStampTagPrefix)
		tsEnd := tsStart
		for i := tsStart; i < len(docContent) && i < tsStart+50; i++ {
			if docContent[i] == '-' && i+2 < len(docContent) && docContent[i+1] == '-' && docContent[i+2] == '>' {
				tsEnd = i
				break
			}
		}
		if tsEnd > tsStart {
			timestamp = string(docContent[tsStart:tsEnd])
		}
	}

	// 查找水印标记
	watermarkTagPrefix := "<!-- Watermark: "
	if idx := bytes.Index(docContent, []byte(watermarkTagPrefix)); idx > 0 {
		start := idx + len(watermarkTagPrefix)
		end := start
		for i := start; i < len(docContent) && i < start+100; i++ {
			if docContent[i] == '-' && i+2 < len(docContent) && docContent[i+1] == '-' && docContent[i+2] == '>' {
				end = i
				break
			}
		}
		if end > start {
			return string(docContent[start:end]), timestamp, nil
		}
	}

	return "", "", fmt.Errorf("未找到水印信息")
}

// GetSupportedType 返回支持的文件类型
func (d *DOCXWatermarker) GetSupportedType() string {
	return "docx"
}

// unzipFile 解压文件到指定目录
func unzipFile(zipFile, destDir string) error {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)

		// 创建目录
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		// 解压文件
		fileReader, err := file.Open()
		if err != nil {
			return err
		}

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fileReader.Close()
			return err
		}

		_, err = io.Copy(targetFile, fileReader)
		targetFile.Close()
		fileReader.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// zipDir 压缩目录为ZIP文件
func zipDir(sourceDir, zipFile string) error {
	// 创建ZIP文件
	file, err := os.Create(zipFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	// 遍历源目录
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对路径
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// 跳过根目录本身
		if relPath == "." {
			return nil
		}

		// 使用标准的路径分隔符
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// 处理目录
		if info.IsDir() {
			_, err = writer.Create(relPath + "/")
			return err
		}

		// 处理文件
		fileToZip, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fileToZip.Close()

		// 创建ZIP内的文件
		zipEntry, err := writer.Create(relPath)
		if err != nil {
			return err
		}

		// 复制文件内容
		_, err = io.Copy(zipEntry, fileToZip)
		return err
	})
}
