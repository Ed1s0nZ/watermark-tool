package png

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"os"
	"strings"
	"time"

	"watermark-tool/internal/watermark"
)

func init() {
	watermark.RegisterWatermarker(NewPNGWatermarker())
}

// PNGWatermarker 提供对PNG图片的水印操作
type PNGWatermarker struct{}

// NewPNGWatermarker 创建一个新的PNG水印处理器
func NewPNGWatermarker() *PNGWatermarker {
	return &PNGWatermarker{}
}

// GetSupportedType 获取支持的文件类型
func (w *PNGWatermarker) GetSupportedType() string {
	return "png"
}

// 定义水印标记
const (
	watermarkPrefix = "<!--WATERMARK_BEGIN:"
	watermarkSuffix = ":WATERMARK_END-->"
)

// AddWatermark 添加水印到PNG图片
func (w *PNGWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 打开PNG文件
	file, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("打开图片文件失败: %w", err)
	}
	defer file.Close()

	// 解码PNG图片
	img, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("解码图片失败: %w", err)
	}

	// 创建输出文件
	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer output.Close()

	// 编码水印文本
	encodedText := base64.StdEncoding.EncodeToString([]byte(watermarkText))

	// 添加时间戳
	timestamp := time.Now().Format(time.RFC3339)

	// 创建水印元数据
	metadata := fmt.Sprintf("%s%s|%s%s",
		watermarkPrefix,
		encodedText,
		timestamp,
		watermarkSuffix)

	// 在不影响图像质量的情况下将水印编码到PNG文件中
	// 首先将图像编码到内存缓冲区
	var pngBuffer bytes.Buffer
	if err := png.Encode(&pngBuffer, img); err != nil {
		return fmt.Errorf("编码图片失败: %w", err)
	}

	// 获取PNG数据
	pngData := pngBuffer.Bytes()

	// 创建输出缓冲区
	var outputBuffer bytes.Buffer

	// 写入PNG数据
	outputBuffer.Write(pngData)

	// 在文件末尾添加水印信息作为注释
	outputBuffer.WriteString("\n")
	outputBuffer.WriteString(metadata)
	outputBuffer.WriteString("\n")

	// 写入最终数据到输出文件
	_, err = output.Write(outputBuffer.Bytes())
	if err != nil {
		return fmt.Errorf("写入输出文件失败: %w", err)
	}

	return nil
}

// ExtractWatermark 从PNG图片中提取水印
func (w *PNGWatermarker) ExtractWatermark(inputFile string) (string, string, error) {
	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 读取图片文件
	pngData, err := os.ReadFile(inputFile)
	if err != nil {
		return "", "", fmt.Errorf("读取图片文件失败: %w", err)
	}

	// 将二进制数据转换为字符串以便搜索水印标记
	dataStr := string(pngData)

	// 查找水印信息
	startIdx := strings.Index(dataStr, watermarkPrefix)
	if startIdx == -1 {
		return "", "", errors.New("未找到水印信息")
	}

	endIdx := strings.Index(dataStr[startIdx:], watermarkSuffix)
	if endIdx == -1 {
		return "", "", errors.New("水印信息格式无效")
	}

	// 提取水印数据
	watermarkData := dataStr[startIdx+len(watermarkPrefix) : startIdx+endIdx]

	// 解析水印数据
	parts := strings.Split(watermarkData, "|")
	if len(parts) < 2 {
		return "", "", errors.New("水印数据格式无效")
	}

	encodedText := parts[0]
	timestamp := parts[1]

	// 解码Base64编码的水印文本
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedText)
	if err != nil {
		return "", "", fmt.Errorf("解码水印信息失败: %w", err)
	}

	return string(decodedBytes), timestamp, nil
}
