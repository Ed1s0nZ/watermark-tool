package jpg

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"strings"
	"time"

	"watermark-tool/internal/watermark"
)

func init() {
	watermark.RegisterWatermarker(NewJPGWatermarker())
}

// JPGWatermarker 提供对JPG图片的水印操作
type JPGWatermarker struct{}

// NewJPGWatermarker 创建一个新的JPG水印处理器
func NewJPGWatermarker() *JPGWatermarker {
	return &JPGWatermarker{}
}

// GetSupportedType 获取支持的文件类型
func (w *JPGWatermarker) GetSupportedType() string {
	return "jpg"
}

// WatermarkMetadata 存储水印元数据
type WatermarkMetadata struct {
	Timestamp int64  `json:"timestamp"`
	Checksum  string `json:"checksum"`
	Content   string `json:"content"`
}

// AddWatermark 添加水印到JPG图片
func (w *JPGWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 打开JPG文件
	file, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("打开图片文件失败: %w", err)
	}
	defer file.Close()

	// 解码JPG图片
	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("解码图片失败: %w", err)
	}

	// 创建输出文件
	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer output.Close()

	// 加密水印文本
	encryptedText, err := encryptWatermark(watermarkText)
	if err != nil {
		return fmt.Errorf("加密水印失败: %w", err)
	}

	// 准备水印元数据
	metadata := WatermarkMetadata{
		Timestamp: time.Now().Unix(),
		Checksum:  generateChecksum(watermarkText),
		Content:   encryptedText,
	}

	// 将元数据序列化为JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("序列化水印元数据失败: %w", err)
	}

	// 将元数据编码为Base64
	metadataBase64 := base64.StdEncoding.EncodeToString(metadataJSON)

	// 创建带有水印注释的缓冲区
	var jpegBuffer bytes.Buffer
	err = jpeg.Encode(&jpegBuffer, img, &jpeg.Options{Quality: 95})
	if err != nil {
		return fmt.Errorf("编码图片失败: %w", err)
	}

	// 提取JPEG头部（直到SOI标记之后）
	jpegData := jpegBuffer.Bytes()
	headerEnd := 2 // JPEG文件头始终是FF D8

	// 构建EXIF或Adobe XMP注释段
	// JPEG注释段以标记FF FE开始，然后是2字节长度（包括长度字段本身）
	comment := []byte{0xFF, 0xFE}

	// 注释内容前缀
	prefix := "WATERMARK:"
	commentContent := prefix + metadataBase64

	// 注释长度（包括长度字段的2个字节）
	commentLength := len(commentContent) + 2
	comment = append(comment, byte(commentLength>>8), byte(commentLength&0xFF))
	comment = append(comment, []byte(commentContent)...)

	// 构建最终图片数据：头部 + 注释段 + 剩余数据
	finalData := make([]byte, headerEnd+len(comment)+len(jpegData)-headerEnd)
	copy(finalData[:headerEnd], jpegData[:headerEnd])
	copy(finalData[headerEnd:], comment)
	copy(finalData[headerEnd+len(comment):], jpegData[headerEnd:])

	// 写入最终数据到输出文件
	_, err = output.Write(finalData)
	if err != nil {
		return fmt.Errorf("写入输出文件失败: %w", err)
	}

	return nil
}

// ExtractWatermark 从JPG图片中提取水印
func (w *JPGWatermarker) ExtractWatermark(inputFile string) (string, error) {
	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 读取图片文件
	jpegData, err := os.ReadFile(inputFile)
	if err != nil {
		return "", fmt.Errorf("读取图片文件失败: %w", err)
	}

	// 检查是否为JPEG文件
	if len(jpegData) < 2 || jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		return "", errors.New("无效的JPEG文件格式")
	}

	// 查找注释段（标记FF FE）
	var commentData []byte
	i := 2
	for i < len(jpegData)-4 {
		if jpegData[i] == 0xFF && jpegData[i+1] == 0xFE {
			// 读取注释长度（包括长度字段自身的2字节）
			length := int(jpegData[i+2])<<8 | int(jpegData[i+3])
			if i+4+length-2 <= len(jpegData) {
				comment := string(jpegData[i+4 : i+2+length])
				if strings.HasPrefix(comment, "WATERMARK:") {
					commentData = []byte(strings.TrimPrefix(comment, "WATERMARK:"))
					break
				}
			}
			i += 2 + length
		} else {
			i++
		}
	}

	if commentData == nil {
		return "", errors.New("未找到水印数据")
	}

	// 解码Base64
	metadataJSON, err := base64.StdEncoding.DecodeString(string(commentData))
	if err != nil {
		return "", fmt.Errorf("解码水印元数据失败: %w", err)
	}

	// 解析元数据
	var metadata WatermarkMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return "", fmt.Errorf("解析水印元数据失败: %w", err)
	}

	// 解密水印内容
	watermarkText, err := decryptWatermark(metadata.Content)
	if err != nil {
		return "", fmt.Errorf("解密水印失败: %w", err)
	}

	// 验证校验和
	if generateChecksum(watermarkText) != metadata.Checksum {
		return "", errors.New("水印校验和不匹配，文件可能被篡改")
	}

	return watermarkText, nil
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
