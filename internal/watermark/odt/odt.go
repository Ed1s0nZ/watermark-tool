package odt

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 打开ODT文件（实际上是一个ZIP文件）
	reader, err := zip.OpenReader(inputFile)
	if err != nil {
		return fmt.Errorf("打开ODT文件失败: %w", err)
	}
	defer reader.Close()

	// 创建输出文件
	outputZip, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outputZip.Close()

	// 创建一个新的zip写入器
	zipWriter := zip.NewWriter(outputZip)
	defer zipWriter.Close()

	// 处理元数据：加密水印信息
	encryptedWatermark, err := encryptWatermark(watermarkText)
	if err != nil {
		return fmt.Errorf("加密水印失败: %w", err)
	}

	// 创建元数据文件
	metaEntryName := "watermark-data.xml"
	metaEntry, err := zipWriter.Create(metaEntryName)
	if err != nil {
		return fmt.Errorf("创建水印元数据文件失败: %w", err)
	}

	// 生成水印元数据
	metaData := fmt.Sprintf("<watermark timestamp=\"%d\" checksum=\"%s\">%s</watermark>",
		time.Now().Unix(),
		generateChecksum(watermarkText),
		encryptedWatermark)

	// 写入水印元数据
	if _, err := metaEntry.Write([]byte(metaData)); err != nil {
		return fmt.Errorf("写入水印元数据失败: %w", err)
	}

	// 处理所有现有文件
	for _, file := range reader.File {
		// 不处理已存在的水印元数据
		if strings.HasSuffix(file.Name, "watermark-data.xml") {
			continue
		}

		// 复制原始文件内容
		err := copyZipFile(file, zipWriter)
		if err != nil {
			return fmt.Errorf("复制文件内容失败 %s: %w", file.Name, err)
		}
	}

	return nil
}

// ExtractWatermark 从ODT文档中提取水印
func (w *ODTWatermarker) ExtractWatermark(inputFile string) (string, string, error) {
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

// 复制zip文件内容
func copyZipFile(file *zip.File, zipWriter *zip.Writer) error {
	// 打开源文件
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// 创建目标文件
	w, err := zipWriter.Create(file.Name)
	if err != nil {
		return err
	}

	// 复制内容
	_, err = io.Copy(w, rc)
	return err
}

// 加密水印文本
func encryptWatermark(text string) (string, error) {
	// 定义密钥（实际应用中应从安全来源获取）
	key := []byte("watermark-security-key-for-encryption")
	plaintext := []byte(text)

	// 创建一个新的AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// 创建一个新的GCM模式
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 创建随机数作为nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密数据
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	// 将结果编码为base64字符串
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// 解密水印文本
func decryptWatermark(encryptedText string) (string, error) {
	// 定义密钥（必须与加密时使用的相同）
	key := []byte("watermark-security-key-for-encryption")

	// 解码base64字符串
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	// 创建一个新的AES加密块
	block, err := aes.NewCipher(key)
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
