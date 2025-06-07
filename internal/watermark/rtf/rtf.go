package rtf

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"time"

	"watermark-tool/internal/watermark"
)

func init() {
	watermark.RegisterWatermarker(NewRTFWatermarker())
}

// RTFWatermarker 提供对RTF文件的水印操作
type RTFWatermarker struct{}

// NewRTFWatermarker 创建一个新的RTF水印处理器
func NewRTFWatermarker() *RTFWatermarker {
	return &RTFWatermarker{}
}

// GetSupportedType 获取支持的文件类型
func (w *RTFWatermarker) GetSupportedType() string {
	return "rtf"
}

// AddWatermark 添加水印到RTF文档
func (w *RTFWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 读取RTF文件内容
	fileContent, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("读取RTF文件失败: %w", err)
	}

	// 验证RTF格式
	if !bytes.HasPrefix(fileContent, []byte("{\\rtf1")) {
		return errors.New("无效的RTF文件格式")
	}

	// 准备水印数据
	watermarkData, err := prepareWatermarkData(watermarkText)
	if err != nil {
		return fmt.Errorf("准备水印数据失败: %w", err)
	}

	// 查找插入位置（在文档信息块之后）
	re := regexp.MustCompile(`(?i)\\info[\s\S]*?}`)
	match := re.FindIndex(fileContent)

	var modifiedContent []byte
	if match != nil {
		// 在info块之后插入我们的水印注释
		insertPos := match[1]
		modifiedContent = make([]byte, len(fileContent)+len(watermarkData)+2)
		copy(modifiedContent, fileContent[:insertPos])
		copy(modifiedContent[insertPos:], []byte(watermarkData))
		copy(modifiedContent[insertPos+len(watermarkData):], fileContent[insertPos:])
	} else {
		// 如果没有找到info块，在RTF头部之后插入
		headerEnd := bytes.Index(fileContent, []byte("\\deff"))
		if headerEnd == -1 {
			headerEnd = bytes.Index(fileContent, []byte("\\deflang"))
		}
		if headerEnd == -1 {
			// 退化情况：在RTF声明后插入
			headerEnd = len("{\\rtf1")
		}

		modifiedContent = make([]byte, len(fileContent)+len(watermarkData)+2)
		copy(modifiedContent, fileContent[:headerEnd])
		copy(modifiedContent[headerEnd:], []byte(watermarkData))
		copy(modifiedContent[headerEnd+len(watermarkData):], fileContent[headerEnd:])
	}

	// 写入修改后的内容到输出文件
	err = ioutil.WriteFile(outputFile, modifiedContent, 0644)
	if err != nil {
		return fmt.Errorf("写入输出文件失败: %w", err)
	}

	return nil
}

// ExtractWatermark 从RTF文档中提取水印
func (w *RTFWatermarker) ExtractWatermark(inputFile string) (string, string, error) {
	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 读取RTF文件内容
	fileContent, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return "", "", fmt.Errorf("读取RTF文件失败: %w", err)
	}

	// 验证RTF格式
	if !bytes.HasPrefix(fileContent, []byte("{\\rtf1")) {
		return "", "", errors.New("无效的RTF文件格式")
	}

	// 查找水印数据
	re := regexp.MustCompile(`\{\\*\\watermark-data timestamp="(\d+)" checksum="([a-f0-9]+)"\\watermark-content ([A-Za-z0-9+/=]+)\\watermark-end\}`)
	matches := re.FindSubmatch(fileContent)

	if matches == nil || len(matches) < 4 {
		return "", "", errors.New("未找到水印数据")
	}

	// 获取时间戳、校验和和加密的水印内容
	timestamp := string(matches[1])
	checksum := string(matches[2])
	encryptedContent := string(matches[3])

	// 解密水印内容
	watermarkText, err := decryptWatermark(encryptedContent)
	if err != nil {
		return "", "", fmt.Errorf("解密水印失败: %w", err)
	}

	// 验证校验和
	if generateChecksum(watermarkText) != checksum {
		return "", "", errors.New("水印校验和不匹配，文件可能被篡改")
	}

	// 验证时间戳（可选，此处仅确保它是有效的Unix时间戳）
	_, err = fmt.Sscanf(timestamp, "%d", new(int64))
	if err != nil {
		return "", "", errors.New("水印时间戳无效")
	}

	return watermarkText, timestamp, nil
}

// prepareWatermarkData 准备用于插入RTF文档的水印数据
func prepareWatermarkData(watermarkText string) (string, error) {
	// 加密水印文本
	encryptedText, err := encryptWatermark(watermarkText)
	if err != nil {
		return "", err
	}

	// 生成校验和
	checksum := generateChecksum(watermarkText)

	// 格式化为RTF注释格式
	// 在RTF中，{\\*\\注释内容}表示特殊字段或隐藏内容
	return fmt.Sprintf("{\\*\\watermark-data timestamp=\"%d\" checksum=\"%s\"\\watermark-content %s\\watermark-end}",
		time.Now().Unix(),
		checksum,
		encryptedText), nil
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
