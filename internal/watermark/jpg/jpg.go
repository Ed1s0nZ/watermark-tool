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

	// 读取整个文件内容
	jpegData, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("读取图片文件失败: %w", err)
	}

	// 检查是否为JPEG文件
	if len(jpegData) < 2 || jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		return errors.New("无效的JPEG文件格式")
	}

	// 解码JPG图片以验证其有效性
	_, err = file.Seek(0, 0) // 重置文件指针到开始
	if err != nil {
		return fmt.Errorf("重置文件指针失败: %w", err)
	}

	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("解码图片失败: %w", err)
	}

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

	// 注释内容前缀
	prefix := "WATERMARK:"
	commentContent := prefix + metadataBase64

	// 创建一个新的JPEG图像，保留原始图像的质量
	var outputBuffer bytes.Buffer
	err = jpeg.Encode(&outputBuffer, img, &jpeg.Options{Quality: 95})
	if err != nil {
		return fmt.Errorf("编码图片失败: %w", err)
	}

	// 获取新编码的JPEG数据
	newJpegData := outputBuffer.Bytes()

	// 创建输出文件
	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer output.Close()

	// 写入JPEG文件头 (FF D8)
	_, err = output.Write(newJpegData[:2])
	if err != nil {
		return fmt.Errorf("写入JPEG头部失败: %w", err)
	}

	// 构建注释段
	// JPEG注释段以标记FF FE开始，然后是2字节长度（包括长度字段本身）
	commentLength := len(commentContent) + 2 // +2 是长度字段自身

	// 检查长度是否超过最大值（2字节能表示的最大值）
	if commentLength > 65535 {
		return fmt.Errorf("水印数据太长，无法添加到JPEG注释中")
	}

	// 写入注释标记和长度
	commentHeader := []byte{0xFF, 0xFE, byte(commentLength >> 8), byte(commentLength & 0xFF)}
	_, err = output.Write(commentHeader)
	if err != nil {
		return fmt.Errorf("写入注释头部失败: %w", err)
	}

	// 写入注释内容
	_, err = output.Write([]byte(commentContent))
	if err != nil {
		return fmt.Errorf("写入注释内容失败: %w", err)
	}

	// 写入剩余的JPEG数据（跳过原始的FF D8头部）
	_, err = output.Write(newJpegData[2:])
	if err != nil {
		return fmt.Errorf("写入JPEG数据失败: %w", err)
	}

	return nil
}

// ExtractWatermark 从JPG图片中提取水印
func (w *JPGWatermarker) ExtractWatermark(inputFile string) (string, string, error) {
	// 检查输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 读取图片文件
	jpegData, err := os.ReadFile(inputFile)
	if err != nil {
		return "", "", fmt.Errorf("读取图片文件失败: %w", err)
	}

	// 检查是否为JPEG文件
	if len(jpegData) < 2 || jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		return "", "", errors.New("无效的JPEG文件格式")
	}

	// 打印调试信息
	fmt.Printf("JPEG文件大小: %d 字节\n", len(jpegData))

	// 查找注释段（标记FF FE）
	var commentData []byte
	i := 2 // 跳过文件头的FF D8
	for i < len(jpegData)-4 {
		// 检查是否为段标记（所有段都以FF开始）
		if jpegData[i] == 0xFF {
			segmentType := jpegData[i+1]
			fmt.Printf("在位置 %d 找到段标记: 0x%02X\n", i, segmentType)

			// 检查是否为注释段 (0xFE)
			if segmentType == 0xFE {
				// 读取注释长度（包括长度字段自身的2字节）
				length := int(jpegData[i+2])<<8 | int(jpegData[i+3])
				fmt.Printf("注释段长度: %d 字节\n", length)

				if i+4+length-2 <= len(jpegData) {
					// 提取注释内容（不包括长度字段）
					comment := string(jpegData[i+4 : i+2+length])
					fmt.Printf("注释内容前缀: %s\n", comment[:min(20, len(comment))])

					if strings.HasPrefix(comment, "WATERMARK:") {
						commentData = []byte(strings.TrimPrefix(comment, "WATERMARK:"))
						fmt.Printf("找到水印数据，长度: %d 字节\n", len(commentData))
						break
					}
				}
				// 跳到下一个段
				i += 2 + length
			} else if segmentType >= 0xE0 && segmentType <= 0xEF {
				// APP段 (APP0-APP15)
				if i+4 < len(jpegData) {
					length := int(jpegData[i+2])<<8 | int(jpegData[i+3])
					i += 2 + length
				} else {
					i += 2
				}
			} else if segmentType == 0xDA {
				// 扫描行开始 (SOS)，之后是图像数据，不再有元数据段
				break
			} else {
				// 其他段，读取长度并跳过
				if i+4 < len(jpegData) {
					length := int(jpegData[i+2])<<8 | int(jpegData[i+3])
					i += 2 + length
				} else {
					i += 2
				}
			}
		} else {
			i++
		}
	}

	if commentData == nil {
		return "", "", errors.New("未找到水印数据")
	}

	// 解码Base64
	metadataJSON, err := base64.StdEncoding.DecodeString(string(commentData))
	if err != nil {
		return "", "", fmt.Errorf("解码水印元数据失败: %w", err)
	}

	// 解析元数据
	var metadata WatermarkMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return "", "", fmt.Errorf("解析水印元数据失败: %w", err)
	}

	// 解密水印内容
	watermarkText, err := decryptWatermark(metadata.Content)
	if err != nil {
		return "", "", fmt.Errorf("解密水印失败: %w", err)
	}

	// 验证校验和
	if generateChecksum(watermarkText) != metadata.Checksum {
		return "", "", errors.New("水印校验和不匹配，文件可能被篡改")
	}

	// 将时间戳转换为字符串
	timestamp := time.Unix(metadata.Timestamp, 0).Format(time.RFC3339)

	return watermarkText, timestamp, nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 加密水印文本
func encryptWatermark(text string) (string, error) {
	// 定义密钥（实际应用中应从安全来源获取）
	// 使用固定长度的密钥（32字节/256位）
	originalKey := "watermark-security-key-for-encryption"
	// 确保密钥长度为32字节（AES-256）
	key := make([]byte, 32)
	// 复制原始密钥，如果原始密钥较短则重复使用，较长则截断
	for i := 0; i < 32; i++ {
		key[i] = originalKey[i%len(originalKey)]
	}

	plaintext := []byte(text)

	// 创建一个新的AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建AES加密块失败: %w", err)
	}

	// 创建一个新的GCM模式
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
	}

	// 创建随机数作为nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}

	// 加密数据
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	// 将结果编码为base64字符串
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// 解密水印文本
func decryptWatermark(encryptedText string) (string, error) {
	// 定义密钥（必须与加密时使用的相同）
	// 使用固定长度的密钥（32字节/256位）
	originalKey := "watermark-security-key-for-encryption"
	// 确保密钥长度为32字节（AES-256）
	key := make([]byte, 32)
	// 复制原始密钥，如果原始密钥较短则重复使用，较长则截断
	for i := 0; i < 32; i++ {
		key[i] = originalKey[i%len(originalKey)]
	}

	// 解码base64字符串
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", fmt.Errorf("base64解码失败: %w", err)
	}

	// 创建一个新的AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建AES加密块失败: %w", err)
	}

	// 创建一个新的GCM模式
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
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
		return "", fmt.Errorf("解密数据失败: %w", err)
	}

	return string(plaintext), nil
}

// 生成校验和
func generateChecksum(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
