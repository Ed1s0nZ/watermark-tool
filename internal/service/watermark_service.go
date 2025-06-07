package service

import (
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"watermark-tool/internal/watermark"
)

// 定义支持的最大文件大小（50MB）
const MaxFileSize = 50 * 1024 * 1024

// 定义错误类型
var (
	ErrFileTooBig      = errors.New("文件大小超过限制")
	ErrInvalidFileType = errors.New("不支持的文件类型")
	ErrEmptyWatermark  = errors.New("水印文本不能为空")
	ErrFileNotFound    = errors.New("文件不存在")
	ErrFileCorrupted   = errors.New("文件已损坏或格式不正确")
)

// WatermarkService 提供水印操作服务
type WatermarkService struct{}

// NewWatermarkService 创建一个新的水印服务
func NewWatermarkService() *WatermarkService {
	return &WatermarkService{}
}

// validateFile 验证文件是否符合要求
func (s *WatermarkService) validateFile(filePath string) error {
	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFound
		}
		return fmt.Errorf("检查文件失败: %w", err)
	}

	// 检查文件大小
	if info.Size() > MaxFileSize {
		return ErrFileTooBig
	}

	// 检查文件类型
	fileExt := strings.ToLower(filepath.Ext(filePath))
	if fileExt == "" {
		return ErrInvalidFileType
	}
	fileExt = fileExt[1:] // 去掉点号

	// 检查是否支持该文件类型
	_, ok := watermark.GetWatermarker(fileExt)
	if !ok {
		return ErrInvalidFileType
	}

	// 尝试读取文件的前几个字节，确保文件未损坏
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 读取前1KB检查文件是否可读
	buffer := make([]byte, 1024)
	_, err = file.Read(buffer)
	if err != nil {
		return ErrFileCorrupted
	}

	return nil
}

// validateWatermarkText 验证水印文本是否有效
func (s *WatermarkService) validateWatermarkText(text string) error {
	// 检查水印文本是否为空
	if strings.TrimSpace(text) == "" {
		return ErrEmptyWatermark
	}

	// 检查水印文本长度
	if len(text) > 100 {
		return errors.New("水印文本过长，请控制在100个字符以内")
	}

	return nil
}

// AddWatermark 为文档添加水印
func (s *WatermarkService) AddWatermark(inputFile, outputFile, watermarkText string) error {
	// 验证水印文本
	if err := s.validateWatermarkText(watermarkText); err != nil {
		return err
	}

	// 验证输入文件
	if err := s.validateFile(inputFile); err != nil {
		return err
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 根据文件扩展名获取处理器
	fileExt := strings.ToLower(filepath.Ext(inputFile))
	fileExt = fileExt[1:] // 去掉扩展名前面的点号

	// 获取对应的水印处理器
	processor, ok := watermark.GetWatermarker(fileExt)
	if !ok {
		return fmt.Errorf("不支持的文件类型: %s", fileExt)
	}

	// 记录开始时间，用于性能分析
	startTime := time.Now()

	// 添加水印
	err := processor.AddWatermark(inputFile, outputFile, watermarkText)

	// 记录处理时间
	elapsedTime := time.Since(startTime)
	fmt.Printf("处理文件 %s 耗时: %v\n", filepath.Base(inputFile), elapsedTime)

	if err != nil {
		return fmt.Errorf("添加水印失败: %w", err)
	}

	// 验证输出文件是否成功创建
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		return fmt.Errorf("输出文件未创建")
	}

	return nil
}

// ExtractWatermark 从文档中提取水印
func (s *WatermarkService) ExtractWatermark(inputFile string) (string, error) {
	// 验证输入文件
	if err := s.validateFile(inputFile); err != nil {
		return "", err
	}

	// 根据文件扩展名获取处理器
	fileExt := strings.ToLower(filepath.Ext(inputFile))
	fileExt = fileExt[1:] // 去掉扩展名前面的点号

	// 获取对应的水印处理器
	processor, ok := watermark.GetWatermarker(fileExt)
	if !ok {
		return "", fmt.Errorf("不支持的文件类型: %s", fileExt)
	}

	// 记录开始时间，用于性能分析
	startTime := time.Now()

	// 提取水印
	watermarkText, _, err := processor.ExtractWatermark(inputFile)

	// 记录处理时间
	elapsedTime := time.Since(startTime)
	fmt.Printf("提取水印从文件 %s 耗时: %v\n", filepath.Base(inputFile), elapsedTime)

	if err != nil {
		return "", fmt.Errorf("提取水印失败: %w", err)
	}

	return watermarkText, nil
}

// ExtractWatermarkWithTimestamp 从文档中提取水印和时间戳
func (s *WatermarkService) ExtractWatermarkWithTimestamp(inputFile string) (string, string, error) {
	// 验证输入文件
	if err := s.validateFile(inputFile); err != nil {
		return "", "", err
	}

	// 根据文件扩展名获取处理器
	fileExt := strings.ToLower(filepath.Ext(inputFile))
	fileExt = fileExt[1:] // 去掉扩展名前面的点号

	// 获取对应的水印处理器
	processor, ok := watermark.GetWatermarker(fileExt)
	if !ok {
		return "", "", fmt.Errorf("不支持的文件类型: %s", fileExt)
	}

	// 记录开始时间，用于性能分析
	startTime := time.Now()

	// 提取水印
	watermarkText, timestamp, err := processor.ExtractWatermark(inputFile)

	// 记录处理时间
	elapsedTime := time.Since(startTime)
	fmt.Printf("提取水印从文件 %s 耗时: %v\n", filepath.Base(inputFile), elapsedTime)

	if err != nil {
		return "", "", fmt.Errorf("提取水印失败: %w", err)
	}

	return watermarkText, timestamp, nil
}

// GetSupportedTypes 获取所有支持的文件类型
func (s *WatermarkService) GetSupportedTypes() []string {
	types := make([]string, 0, len(watermark.WatermarkRegistry))
	for fileType := range watermark.WatermarkRegistry {
		types = append(types, fileType)
	}
	return types
}

// ValidateMimeType 验证MIME类型是否为支持的文档类型
func (s *WatermarkService) ValidateMimeType(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return ErrInvalidFileType
	}

	// 获取MIME类型
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// 如果无法通过扩展名获取MIME类型，使用扩展名判断
		ext = ext[1:] // 移除点号
		_, ok := watermark.GetWatermarker(ext)
		if !ok {
			return ErrInvalidFileType
		}
		return nil
	}

	// 检查MIME类型是否为支持的文档类型
	validMimeTypes := map[string]bool{
		"application/pdf": true, // PDF
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true, // DOCX
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true, // XLSX
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true, // PPTX
		"application/vnd.oasis.opendocument.text":                                   true, // ODT
		"application/rtf": true, // RTF
		"text/rtf":        true, // RTF 另一种MIME类型
		"image/jpeg":      true, // JPG
		"image/jpg":       true, // JPG 另一种MIME类型
		"image/png":       true, // PNG
	}

	if !validMimeTypes[mimeType] {
		return ErrInvalidFileType
	}

	return nil
}
