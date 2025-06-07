package xlsx

import (
	"archive/zip"
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

// encrypt 使用AES加密文本
func encrypt(plaintext, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	// 创建一个新的初始化向量
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(plaintext))

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt 使用AES解密文本
func decrypt(ciphertext, key string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	if len(data) < aes.BlockSize {
		return "", errors.New("密文太短")
	}

	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(data, data)

	return string(data), nil
}

// calculateChecksum 计算文本的MD5校验和
func calculateChecksum(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// createWatermarkData 创建加密的水印数据
func createWatermarkData(text string) (string, string, error) {
	// 生成时间戳
	timestamp := time.Now().Format(time.RFC3339)

	// 计算校验和
	checksum := calculateChecksum(text + timestamp)

	// 加密水印文本
	encryptedText, err := encrypt(text, checksum[:16])
	if err != nil {
		return "", "", err
	}

	// 格式化水印数据
	watermarkData := fmt.Sprintf("%s%s|%s|%s%s",
		watermarkPrefix,
		encryptedText,
		timestamp,
		checksum,
		watermarkSuffix)

	return watermarkData, checksum, nil
}

// injectWatermarkIntoFile 将水印注入到指定的XML文件中
func injectWatermarkIntoFile(r *zip.Reader, w *zip.Writer, fileName string, watermarkData string) error {
	// 查找XML文件
	var xmlFile *zip.File
	for _, f := range r.File {
		if f.Name == fileName {
			xmlFile = f
			break
		}
	}

	if xmlFile == nil {
		return fmt.Errorf("在XLSX中未找到文件: %s", fileName)
	}

	// 打开XML文件
	rc, err := xmlFile.Open()
	if err != nil {
		return fmt.Errorf("打开XLSX内部文件失败: %w", err)
	}
	defer rc.Close()

	// 读取XML内容
	xmlContent, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("读取XLSX内部文件失败: %w", err)
	}

	var modifiedContent []byte

	switch fileName {
	case corePropsFile:
		// 在core.xml中插入自定义属性
		endTag := "</cp:coreProperties>"
		if idx := bytes.LastIndex(xmlContent, []byte(endTag)); idx > 0 {
			// 添加自定义注释
			comment := fmt.Sprintf("<!-- %s -->", watermarkData)
			modifiedContent = append(xmlContent[:idx], []byte(comment)...)
			modifiedContent = append(modifiedContent, xmlContent[idx:]...)
		} else {
			modifiedContent = xmlContent
		}

	case workbookFile:
		// 在workbook.xml中插入水印
		endTag := "</workbook>"
		if idx := bytes.LastIndex(xmlContent, []byte(endTag)); idx > 0 {
			// 添加自定义注释
			comment := fmt.Sprintf("<!-- %s -->", watermarkData)
			modifiedContent = append(xmlContent[:idx], []byte(comment)...)
			modifiedContent = append(modifiedContent, xmlContent[idx:]...)
		} else {
			modifiedContent = xmlContent
		}

	case sharedStringsFile:
		// 在sharedStrings.xml中添加隐藏的字符串
		endTag := "</sst>"
		if idx := bytes.LastIndex(xmlContent, []byte(endTag)); idx > 0 {
			// 添加隐藏的共享字符串
			hiddenString := fmt.Sprintf(`<si><t xml:space="preserve">%s</t></si>`, watermarkData)
			modifiedContent = append(xmlContent[:idx], []byte(hiddenString)...)
			modifiedContent = append(modifiedContent, xmlContent[idx:]...)
		} else {
			modifiedContent = xmlContent
		}

	default:
		modifiedContent = xmlContent
	}

	// 创建新的ZIP条目
	header := &zip.FileHeader{
		Name:   fileName,
		Method: zip.Deflate,
	}
	header.SetModTime(time.Now())

	// 写入修改后的内容
	writer, err := w.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("创建ZIP条目失败: %w", err)
	}

	_, err = writer.Write(modifiedContent)
	if err != nil {
		return fmt.Errorf("写入ZIP条目失败: %w", err)
	}

	return nil
}

// extractWatermarkFromFile 从指定的XML文件中提取水印
func extractWatermarkFromFile(r *zip.Reader, fileName string) (string, error) {
	// 查找XML文件
	var xmlFile *zip.File
	for _, f := range r.File {
		if f.Name == fileName {
			xmlFile = f
			break
		}
	}

	if xmlFile == nil {
		return "", fmt.Errorf("在XLSX中未找到文件: %s", fileName)
	}

	// 打开XML文件
	rc, err := xmlFile.Open()
	if err != nil {
		return "", fmt.Errorf("打开XLSX内部文件失败: %w", err)
	}
	defer rc.Close()

	// 读取XML内容
	xmlContent, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("读取XLSX内部文件失败: %w", err)
	}

	// 使用正则表达式查找水印数据
	pattern := regexp.MustCompile(watermarkPrefix + `(.*?)` + watermarkSuffix)
	matches := pattern.FindSubmatch(xmlContent)

	if len(matches) < 2 {
		return "", nil // 未在此文件中找到水印，继续查找其他文件
	}

	return string(matches[1]), nil
}

// AddWatermark 为XLSX文件添加水印
func (x *XLSXWatermarker) AddWatermark(inputFile, outputFile, watermarkText string) error {
	// 读取源XLSX文件
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("读取XLSX文件失败: %w", err)
	}

	// 打开ZIP文件
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("解析XLSX文件失败: %w", err)
	}

	// 创建输出缓冲区
	var outputBuffer bytes.Buffer
	writer := zip.NewWriter(&outputBuffer)

	// 创建加密的水印数据
	watermarkData, _, err := createWatermarkData(watermarkText)
	if err != nil {
		return fmt.Errorf("创建水印数据失败: %w", err)
	}

	// 处理所有文件
	processedFiles := make(map[string]bool)

	// 首先处理需要插入水印的文件
	watermarkFiles := []string{corePropsFile, workbookFile, sharedStringsFile}
	for _, fileName := range watermarkFiles {
		err := injectWatermarkIntoFile(reader, writer, fileName, watermarkData)
		if err == nil {
			processedFiles[fileName] = true
		}
		// 忽略错误，如果文件不存在或插入失败，继续处理其他文件
	}

	// 复制其余文件
	for _, file := range reader.File {
		if processedFiles[file.Name] {
			continue // 跳过已处理的文件
		}

		// 打开原始文件
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("打开XLSX内部文件失败: %w", err)
		}

		// 创建新的ZIP条目
		w, err := writer.CreateHeader(&file.FileHeader)
		if err != nil {
			rc.Close()
			return fmt.Errorf("创建ZIP条目失败: %w", err)
		}

		// 复制内容
		_, err = io.Copy(w, rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("复制ZIP条目失败: %w", err)
		}
	}

	// 关闭ZIP写入器
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("关闭ZIP写入器失败: %w", err)
	}

	// 写入输出文件
	err = os.WriteFile(outputFile, outputBuffer.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("写入输出文件失败: %w", err)
	}

	return nil
}

// ExtractWatermark 从XLSX文件中提取水印
func (x *XLSXWatermarker) ExtractWatermark(inputFile string) (string, error) {
	// 读取XLSX文件
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", fmt.Errorf("读取XLSX文件失败: %w", err)
	}

	// 打开ZIP文件
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("解析XLSX文件失败: %w", err)
	}

	// 在可能的位置查找水印
	watermarkFiles := []string{corePropsFile, workbookFile, sharedStringsFile}
	for _, fileName := range watermarkFiles {
		watermarkData, err := extractWatermarkFromFile(reader, fileName)
		if err != nil || watermarkData == "" {
			continue // 如果此文件中没有找到水印，继续查找其他文件
		}

		// 解析水印数据
		parts := strings.Split(watermarkData, "|")
		if len(parts) < 3 {
			continue // 无效的水印格式，尝试下一个文件
		}

		encryptedText := parts[0]
		timestamp := parts[1]
		checksum := parts[2]

		// 解密水印文本
		decryptedText, err := decrypt(encryptedText, checksum[:16])
		if err != nil {
			continue // 解密失败，尝试下一个文件
		}

		// 验证水印完整性
		calculatedChecksum := calculateChecksum(decryptedText + timestamp)
		if calculatedChecksum != checksum {
			continue // 校验和不匹配，尝试下一个文件
		}

		return decryptedText, nil
	}

	return "", errors.New("未在XLSX文件中找到有效的水印信息")
}
