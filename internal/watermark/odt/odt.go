package odt

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"watermark-tool/internal/watermark"
)

func init() {
	watermark.RegisterWatermarker(NewODTWatermarker())
}

// ODTWatermarker 提供对ODT文件的水印操作
type ODTWatermarker struct{}

// NewODTWatermarker 创建一个新的ODT水印处理器
func NewODTWatermarker() *ODTWatermarker {
	return &ODTWatermarker{}
}

// GetSupportedType 获取支持的文件类型
func (w *ODTWatermarker) GetSupportedType() string {
	return "odt"
}

// AddWatermark 添加水印到ODT文档
func (w *ODTWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	return watermark.WriteXMLArchiveMetadata(inputFile, outputFile, []string{"meta.xml", "content.xml"}, watermarkText, time.Now().UTC())
}

// ExtractWatermark 从ODT文档中提取水印
func (w *ODTWatermarker) ExtractWatermark(inputFile string) (string, string, error) {
	if text, at, found, err := watermark.ReadXMLArchiveMetadata(inputFile, []string{"meta.xml", "content.xml"}); err != nil {
		return "", "", err
	} else if found {
		return text, at, nil
	}

	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 打开ODT文件
	reader, err := zip.OpenReader(inputFile)
	if err != nil {
		return "", "", fmt.Errorf("打开ODT文件失败: %w", err)
	}
	defer reader.Close()

	// 查找并读取水印元数据文件
	var watermarkData []byte
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "watermark-data.xml") {
			rc, err := file.Open()
			if err != nil {
				return "", "", fmt.Errorf("打开水印元数据文件失败: %w", err)
			}
			defer rc.Close()

			watermarkData, err = ioutil.ReadAll(rc)
			if err != nil {
				return "", "", fmt.Errorf("读取水印元数据失败: %w", err)
			}
			break
		}
	}

	if watermarkData == nil || len(watermarkData) == 0 {
		return "", "", errors.New("未找到水印数据")
	}

	// 解析XML数据
	var watermarkInfo struct {
		XMLName   xml.Name `xml:"watermark"`
		Timestamp string   `xml:"timestamp,attr"`
		Checksum  string   `xml:"checksum,attr"`
		Data      string   `xml:",chardata"`
	}

	if err := xml.Unmarshal(watermarkData, &watermarkInfo); err != nil {
		return "", "", fmt.Errorf("解析水印元数据失败: %w", err)
	}

	// 解密水印数据
	decryptedWatermark, err := decryptWatermark(watermarkInfo.Data)
	if err != nil {
		return "", "", fmt.Errorf("解密水印失败: %w", err)
	}

	// 验证校验和
	if generateChecksum(decryptedWatermark) != watermarkInfo.Checksum {
		return "", "", errors.New("水印校验和不匹配，文件可能被篡改")
	}

	return decryptedWatermark, watermarkInfo.Timestamp, nil
}

// 解密水印文本
func decryptWatermark(encryptedText string) (string, error) {
	// 定义密钥（必须与加密时使用的相同）
	key := sha256.Sum256([]byte("watermark-security-key-for-encryption"))

	// 解码base64字符串
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	// 创建一个新的AES加密块
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	// 创建一个新的GCM模式
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 获取nonce大小
	nonceSize := aesGCM.NonceSize()

	// 检查密文长度
	if len(ciphertext) < nonceSize {
		return "", errors.New("密文长度不足")
	}

	// 提取nonce和实际密文
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密数据
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// 生成校验和
func generateChecksum(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
