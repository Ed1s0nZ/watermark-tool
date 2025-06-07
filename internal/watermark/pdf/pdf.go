package pdf

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"watermark-tool/internal/watermark"
)

// PDFWatermarker 实现了PDF文件的水印处理
type PDFWatermarker struct{}

// 注册PDF水印处理器
func init() {
	watermark.RegisterWatermarker(&PDFWatermarker{})
}

// GetSupportedType 返回支持的文件类型
func (p *PDFWatermarker) GetSupportedType() string {
	return "pdf"
}

// 定义PDF水印标记
const (
	watermarkPrefix = "%WATERMARK_BEGIN:"
	watermarkSuffix = ":WATERMARK_END%"
)

// createWatermarkMetadata 创建包含水印信息的元数据
func createWatermarkMetadata(text string) string {
	// 生成时间戳
	timestamp := time.Now().Format(time.RFC3339)

	// 简单编码水印文本
	encodedText := base64.StdEncoding.EncodeToString([]byte(text))

	// 格式化元数据
	metadata := fmt.Sprintf("%s%s|%s%s",
		watermarkPrefix,
		encodedText,
		timestamp,
		watermarkSuffix)

	return metadata
}

// insertMetadata 在PDF文件的不同位置插入元数据
func insertMetadata(pdfData []byte, metadata string) []byte {
	var buffer bytes.Buffer

	// 在文件的多个位置插入水印，增加隐蔽性和冗余性

	// 1. 在PDF trailer附近插入
	trailerPos := bytes.LastIndex(pdfData, []byte("trailer"))
	if trailerPos > 0 {
		buffer.Write(pdfData[:trailerPos])
		buffer.WriteString("\n")
		buffer.WriteString(metadata)
		buffer.WriteString("\n")
		buffer.Write(pdfData[trailerPos:])
		return buffer.Bytes()
	}

	// 2. 在文件末尾插入（作为备用方案）
	buffer.Write(pdfData)
	buffer.WriteString("\n")
	buffer.WriteString(metadata)
	buffer.WriteString("\n")

	return buffer.Bytes()
}

// AddWatermark 为PDF文件添加水印
func (p *PDFWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	// 读取源PDF文件
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("读取PDF文件失败: %w", err)
	}

	// 验证是否为PDF文件
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return errors.New("不是有效的PDF文件")
	}

	// 创建包含水印信息的元数据
	metadata := createWatermarkMetadata(watermarkText)

	// 在PDF文件中插入元数据
	watermarkedData := insertMetadata(data, metadata)

	// 写入输出文件
	err = os.WriteFile(outputFile, watermarkedData, 0644)
	if err != nil {
		return fmt.Errorf("写入PDF文件失败: %w", err)
	}

	return nil
}

// ExtractWatermark 从PDF文件中提取水印
func (p *PDFWatermarker) ExtractWatermark(inputFile string) (string, error) {
	// 读取PDF文件
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", fmt.Errorf("读取PDF文件失败: %w", err)
	}

	// 验证是否为PDF文件
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return "", errors.New("不是有效的PDF文件")
	}

	// 使用正则表达式提取水印元数据
	pattern := regexp.MustCompile(watermarkPrefix + `(.*?)` + watermarkSuffix)
	matches := pattern.FindSubmatch(data)

	if len(matches) < 2 {
		return "", errors.New("未找到水印信息")
	}

	// 解析水印元数据
	watermarkData := string(matches[1])
	parts := strings.Split(watermarkData, "|")
	if len(parts) < 2 {
		return "", errors.New("水印格式无效")
	}

	encodedText := parts[0]

	// 解码水印文本
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedText)
	if err != nil {
		return "", fmt.Errorf("解码水印失败: %w", err)
	}

	return string(decodedBytes), nil
}
