package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"watermark-tool/internal/watermark"
)

// PPTXWatermarker 实现PowerPoint文档的水印处理
type PPTXWatermarker struct{}

// 注册PowerPoint水印处理器
func init() {
	watermark.RegisterWatermarker(&PPTXWatermarker{})
}

// AddWatermark 为PowerPoint添加水印
func (p *PPTXWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "pptx-watermark-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压PPTX文件
	err = unzipFile(inputFile, tempDir)
	if err != nil {
		return fmt.Errorf("解压PPTX文件失败: %w", err)
	}

	// 添加水印到元数据
	// 修改core.xml添加水印信息
	corePropsPath := filepath.Join(tempDir, "docProps", "core.xml")
	if _, err := os.Stat(corePropsPath); err == nil {
		coreContent, err := os.ReadFile(corePropsPath)
		if err == nil {
			// 添加水印到关键词
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

	// 在每个幻灯片中添加水印
	// 查找所有幻灯片文件
	slidesDir := filepath.Join(tempDir, "ppt", "slides")
	if _, err := os.Stat(slidesDir); err == nil {
		files, err := os.ReadDir(slidesDir)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && strings.HasPrefix(file.Name(), "slide") && strings.HasSuffix(file.Name(), ".xml") {
					slidePath := filepath.Join(slidesDir, file.Name())

					// 读取幻灯片内容
					slideContent, err := os.ReadFile(slidePath)
					if err != nil {
						continue
					}

					// 在幻灯片末尾添加水印注释
					watermarkComment := fmt.Sprintf("<!-- Watermark: %s -->", watermarkText)
					if !bytes.Contains(slideContent, []byte(watermarkComment)) {
						endTagPos := bytes.LastIndex(slideContent, []byte("</p:sld>"))
						if endTagPos > 0 {
							newContent := append(
								slideContent[:endTagPos],
								append(
									[]byte(watermarkComment),
									slideContent[endTagPos:]...,
								)...,
							)
							err = os.WriteFile(slidePath, newContent, 0644)
							if err != nil {
								return fmt.Errorf("写入幻灯片水印失败: %w", err)
							}
						}
					}
				}
			}
		}
	}

	// 重新打包PPTX文件
	err = zipDir(tempDir, outputFile)
	if err != nil {
		return fmt.Errorf("重新打包PPTX文件失败: %w", err)
	}

	return nil
}

// ExtractWatermark 从PowerPoint中提取水印
func (p *PPTXWatermarker) ExtractWatermark(inputFile string) (string, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "pptx-extract-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压PPTX文件
	err = unzipFile(inputFile, tempDir)
	if err != nil {
		return "", fmt.Errorf("解压PPTX文件失败: %w", err)
	}

	// 首先检查文档属性
	corePropsPath := filepath.Join(tempDir, "docProps", "core.xml")
	if _, err := os.Stat(corePropsPath); err == nil {
		coreContent, err := os.ReadFile(corePropsPath)
		if err == nil {
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
					return string(coreContent[start:end]), nil
				}
			}
		}
	}

	// 如果在文档属性中没找到，在幻灯片中查找
	slidesDir := filepath.Join(tempDir, "ppt", "slides")
	if _, err := os.Stat(slidesDir); err == nil {
		files, err := os.ReadDir(slidesDir)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && strings.HasPrefix(file.Name(), "slide") && strings.HasSuffix(file.Name(), ".xml") {
					slidePath := filepath.Join(slidesDir, file.Name())

					// 读取幻灯片内容
					slideContent, err := os.ReadFile(slidePath)
					if err != nil {
						continue
					}

					// 查找水印注释
					commentPrefix := "<!-- Watermark: "
					if idx := bytes.Index(slideContent, []byte(commentPrefix)); idx > 0 {
						start := idx + len(commentPrefix)
						end := start
						for i := start; i < len(slideContent) && i < start+100; i++ {
							if slideContent[i] == '-' && i+2 < len(slideContent) && slideContent[i+1] == '-' && slideContent[i+2] == '>' {
								end = i
								break
							}
						}
						if end > start {
							return string(slideContent[start:end]), nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("未找到水印信息")
}

// GetSupportedType 返回支持的文件类型
func (p *PPTXWatermarker) GetSupportedType() string {
	return "pptx"
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
