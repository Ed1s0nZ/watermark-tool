package xlsx

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"watermark-tool/internal/watermark"
)

// XLSXWatermarker 实现了XLSX文件的水印处理
type XLSXWatermarker struct{}

// 注册XLSX水印处理器
func init() {
	watermark.RegisterWatermarker(&XLSXWatermarker{})
}

// GetSupportedType 返回支持的文件类型
func (x *XLSXWatermarker) GetSupportedType() string {
	return "xlsx"
}

// 定义XLSX水印标记
const (
	watermarkPrefix = "WATERMARK_BEGIN:"
	watermarkSuffix = ":WATERMARK_END"
)

// 定义可以插入水印的XLSX文件内的位置
const (
	corePropsFile     = "docProps/core.xml"
	workbookFile      = "xl/workbook.xml"
	sharedStringsFile = "xl/sharedStrings.xml"
)

// decrypt 使用AES解密文本
func decrypt(ciphertext, key string) (string, error) {

	// 尝试多种解码方式
	var data []byte
	var err error

	// 首先尝试RawURLEncoding（我们现在使用的编码方式）
	data, err = base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		// 尝试URLEncoding
		data, err = base64.URLEncoding.DecodeString(ciphertext)
		if err != nil {
			// 尝试StdEncoding（旧版本使用的编码）
			data, err = base64.StdEncoding.DecodeString(ciphertext)
			if err != nil {
				// 尝试RawStdEncoding
				data, err = base64.RawStdEncoding.DecodeString(ciphertext)
				if err != nil {
					// 尝试修复常见的base64问题后再解码
					fixedText := strings.TrimSpace(ciphertext)
					fixedText = strings.Replace(fixedText, " ", "+", -1)
					fixedText = strings.Replace(fixedText, "-", "+", -1)
					fixedText = strings.Replace(fixedText, "_", "/", -1)

					// 确保长度是4的倍数
					padding := len(fixedText) % 4
					if padding > 0 {
						fixedText += strings.Repeat("=", 4-padding)
					}

					data, err = base64.StdEncoding.DecodeString(fixedText)
					if err != nil {
						return "", fmt.Errorf("所有base64解码方法都失败: %w", err)
					}
				}
			}
		}
	}

	// 确保密钥长度正确
	if len(key) < 16 {
		return "", fmt.Errorf("密钥长度不足: %d", len(key))
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("密文太短: %d", len(data))
	}

	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(data, data)

	return string(data), nil
}

// AddWatermark 为XLSX文件添加水印
func (x *XLSXWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	return watermark.WriteXMLArchiveMetadata(inputFile, outputFile, []string{corePropsFile, workbookFile}, watermarkText, time.Now().UTC())
}

// ExtractWatermark 从XLSX文件中提取水印
func (x *XLSXWatermarker) ExtractWatermark(inputFile string) (string, string, error) {
	if text, at, found, err := watermark.ReadXMLArchiveMetadata(inputFile, []string{corePropsFile, workbookFile}); err != nil {
		return "", "", err
	} else if found {
		return text, at, nil
	}

	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", "", fmt.Errorf("读取XLSX文件失败: %w", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", "", fmt.Errorf("解析XLSX文件失败: %w", err)
	}
	const customPropsFile = "docProps/custom.xml"
	for _, f := range reader.File {
		if f.Name == customPropsFile {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			// 提取自定义属性
			pattern := regexp.MustCompile(`<property[^>]*name="watermark"[^>]*>(.*?)</property>`)
			matches := pattern.FindSubmatch(content)
			if len(matches) >= 2 {
				watermarkData := string(matches[1])
				return parseWatermarkData(watermarkData)
			}
		}
	}
	// 兼容旧方法
	return "", "", errors.New("未在XLSX文件中找到有效的水印信息")
}

// parseWatermarkData 解析水印数据
func parseWatermarkData(watermarkData string) (string, string, error) {

	// 首先检查并移除水印前缀和后缀
	if strings.Contains(watermarkData, watermarkPrefix) && strings.Contains(watermarkData, watermarkSuffix) {
		start := strings.Index(watermarkData, watermarkPrefix) + len(watermarkPrefix)
		end := strings.Index(watermarkData, watermarkSuffix)
		if end > start {
			watermarkData = watermarkData[start:end]
		}
	}

	// 解析水印数据
	parts := strings.Split(watermarkData, "|")
	if len(parts) < 3 {
		// 如果格式不正确，尝试更宽松的解析
		if len(watermarkData) > 20 { // 假设至少有一些有效数据
			// 尝试直接解密
			tempKey := "0123456789abcdef"
			decryptedText, err := decrypt(watermarkData, tempKey)
			if err == nil {
				return decryptedText, "", nil
			}
		}
		return "", "", errors.New("无效的水印格式")
	}

	encryptedText := parts[0]
	timestamp := parts[1]
	checksum := parts[2]

	// 确保校验和长度足够
	if len(checksum) < 16 {
		return "", "", fmt.Errorf("校验和长度不足: %d", len(checksum))
	}

	// 解密水印文本
	decryptedText, err := decrypt(encryptedText, checksum[:16])
	if err != nil {
		// 如果使用校验和解密失败，尝试使用固定密钥（不安全，但可能有助于恢复数据）
		tempKey := "0123456789abcdef"
		decryptedText, err = decrypt(encryptedText, tempKey)
		if err != nil {
			return "", "", fmt.Errorf("解密水印失败: %w", err)
		}
		// 解密成功但使用了临时密钥，跳过校验和验证
		return decryptedText, timestamp, nil
	}

	// Legacy CFB records are recoverable compatibility data, not issuer authentication.
	return decryptedText, timestamp, nil
}
