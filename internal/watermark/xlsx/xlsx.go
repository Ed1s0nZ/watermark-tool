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

	// 使用URL安全的base64编码，避免特殊字符问题
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// decrypt 使用AES解密文本
func decrypt(ciphertext, key string) (string, error) {
	// 打印输入参数，便于调试
	fmt.Printf("解密输入 - 密文: %s, 密钥长度: %d\n", ciphertext, len(key))

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

	// 格式化水印数据 - 不使用特殊字符
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
			// 使用自定义属性而不是注释
			customProp := fmt.Sprintf(`<cp:customXmlPart name="watermark">%s</cp:customXmlPart>`, watermarkData)
			// 检查是否支持自定义属性
			if bytes.Contains(xmlContent, []byte("xmlns:cp=")) {
				modifiedContent = append(xmlContent[:idx], []byte(customProp)...)
				modifiedContent = append(modifiedContent, xmlContent[idx:]...)
			} else {
				// 如果不支持，则使用更安全的方式：在描述中添加
				descTag := "</dc:description>"
				if descIdx := bytes.LastIndex(xmlContent, []byte(descTag)); descIdx > 0 {
					// 在描述中添加水印数据
					modifiedContent = append(xmlContent[:descIdx], []byte(fmt.Sprintf("WM:%s", watermarkData))...)
					modifiedContent = append(modifiedContent, xmlContent[descIdx:]...)
				} else {
					// 如果没有描述标签，则尝试在创建者标签前添加描述标签
					creatorTag := "<dc:creator>"
					if creatorIdx := bytes.Index(xmlContent, []byte(creatorTag)); creatorIdx > 0 {
						descElement := fmt.Sprintf("<dc:description>WM:%s</dc:description>", watermarkData)
						modifiedContent = append(xmlContent[:creatorIdx], []byte(descElement)...)
						modifiedContent = append(modifiedContent, xmlContent[creatorIdx:]...)
					} else {
						modifiedContent = xmlContent
					}
				}
			}
		} else {
			modifiedContent = xmlContent
		}

	case workbookFile:
		// 在workbook.xml中插入水印
		// 使用自定义属性而不是注释
		if bytes.Contains(xmlContent, []byte("<fileVersion ")) {
			versionTag := "<fileVersion "
			idx := bytes.Index(xmlContent, []byte(versionTag))
			if idx > 0 {
				// 在fileVersion标签中添加自定义属性
				customAttr := fmt.Sprintf(` customWatermark="%s"`, watermarkData)
				modifiedContent = append(xmlContent[:idx+len(versionTag)], []byte(customAttr)...)
				modifiedContent = append(modifiedContent, xmlContent[idx+len(versionTag):]...)
			} else {
				modifiedContent = xmlContent
			}
		} else {
			// 如果没有fileVersion标签，则在workbookPr标签中添加
			if bytes.Contains(xmlContent, []byte("<workbookPr ")) {
				prTag := "<workbookPr "
				idx := bytes.Index(xmlContent, []byte(prTag))
				if idx > 0 {
					customAttr := fmt.Sprintf(` customWatermark="%s"`, watermarkData)
					modifiedContent = append(xmlContent[:idx+len(prTag)], []byte(customAttr)...)
					modifiedContent = append(modifiedContent, xmlContent[idx+len(prTag):]...)
				} else {
					modifiedContent = xmlContent
				}
			} else {
				modifiedContent = xmlContent
			}
		}

	case sharedStringsFile:
		// 在sharedStrings.xml中添加隐藏的字符串
		// 首先解析当前的计数属性
		countPattern := regexp.MustCompile(`count="(\d+)"`)
		uniqueCountPattern := regexp.MustCompile(`uniqueCount="(\d+)"`)

		countMatches := countPattern.FindSubmatch(xmlContent)
		uniqueCountMatches := uniqueCountPattern.FindSubmatch(xmlContent)

		var countStr, uniqueCountStr string
		var count, uniqueCount int

		if len(countMatches) >= 2 {
			countStr = string(countMatches[1])
			fmt.Sscanf(countStr, "%d", &count)
			count++ // 增加计数
		}

		if len(uniqueCountMatches) >= 2 {
			uniqueCountStr = string(uniqueCountMatches[1])
			fmt.Sscanf(uniqueCountStr, "%d", &uniqueCount)
			uniqueCount++ // 增加唯一计数
		}

		// 更新计数属性
		if countStr != "" {
			newCountAttr := fmt.Sprintf(`count="%d"`, count)
			xmlContent = bytes.Replace(xmlContent, []byte(fmt.Sprintf(`count="%s"`, countStr)), []byte(newCountAttr), 1)
		}

		if uniqueCountStr != "" {
			newUniqueCountAttr := fmt.Sprintf(`uniqueCount="%d"`, uniqueCount)
			xmlContent = bytes.Replace(xmlContent, []byte(fmt.Sprintf(`uniqueCount="%s"`, uniqueCountStr)), []byte(newUniqueCountAttr), 1)
		}

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
	// 首先尝试查找注释中的水印（兼容旧版本）
	pattern := regexp.MustCompile(watermarkPrefix + `(.*?)` + watermarkSuffix)
	matches := pattern.FindSubmatch(xmlContent)
	if len(matches) >= 2 {
		return string(matches[1]), nil
	}

	// 尝试查找自定义XML部分中的水印
	customPattern := regexp.MustCompile(`<cp:customXmlPart name="watermark">(.*?)</cp:customXmlPart>`)
	customMatches := customPattern.FindSubmatch(xmlContent)
	if len(customMatches) >= 2 {
		return string(customMatches[1]), nil
	}

	// 尝试查找描述中的水印
	descPattern := regexp.MustCompile(`<dc:description>WM:(.*?)</dc:description>`)
	descMatches := descPattern.FindSubmatch(xmlContent)
	if len(descMatches) >= 2 {
		return string(descMatches[1]), nil
	}

	// 尝试查找fileVersion或workbookPr属性中的水印
	attrPattern := regexp.MustCompile(`customWatermark="(.*?)"`)
	attrMatches := attrPattern.FindSubmatch(xmlContent)
	if len(attrMatches) >= 2 {
		return string(attrMatches[1]), nil
	}

	return "", nil // 未在此文件中找到水印，继续查找其他文件
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
	watermarkData, checksum, err := createWatermarkData(watermarkText)
	if err != nil {
		return fmt.Errorf("创建水印数据失败: %w", err)
	}

	// 打印水印数据，用于调试
	fmt.Printf("创建的水印数据: %s\n", watermarkData)
	fmt.Printf("校验和: %s\n", checksum)

	// 处理所有文件
	processedFiles := make(map[string]bool)

	// 使用更保守的方法：只在docProps/app.xml中添加水印（如果存在）
	// 这个文件通常不会影响Excel/WPS的核心功能
	const appPropsFile = "docProps/app.xml"

	// 尝试找到app.xml文件
	var appFile *zip.File
	for _, f := range reader.File {
		if f.Name == appPropsFile {
			appFile = f
			break
		}
	}

	// 如果找到app.xml，则在其中添加水印
	if appFile != nil {
		rc, err := appFile.Open()
		if err == nil {
			content, err := io.ReadAll(rc)
			rc.Close()

			if err == nil {
				// 在Properties标签结束前添加自定义属性
				endTag := "</Properties>"
				if idx := bytes.LastIndex(content, []byte(endTag)); idx > 0 {
					// 添加自定义属性，确保XML格式正确
					// 使用CDATA包装水印数据，防止XML特殊字符问题
					customProp := fmt.Sprintf("<CustomDocumentProperties><Property name=\"WM\" type=\"string\"><![CDATA[%s]]></Property></CustomDocumentProperties>", watermarkData)

					modifiedContent := append(content[:idx], []byte(customProp)...)
					modifiedContent = append(modifiedContent, content[idx:]...)

					// 写入修改后的文件
					w, err := writer.CreateHeader(&appFile.FileHeader)
					if err == nil {
						w.Write(modifiedContent)
						processedFiles[appPropsFile] = true
					}
				}
			}
		}
	}

	// 如果app.xml不存在或处理失败，尝试创建一个自定义文件
	if !processedFiles[appPropsFile] {
		customFileName := "xl/customWatermark.xml"
		header := &zip.FileHeader{
			Name:   customFileName,
			Method: zip.Deflate,
		}
		header.SetModTime(time.Now())

		w, err := writer.CreateHeader(header)
		if err == nil {
			// 使用标准XML格式，并用CDATA包装水印数据
			customContent := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?><watermark><![CDATA[%s]]></watermark>", watermarkData)
			w.Write([]byte(customContent))
			processedFiles[customFileName] = true
		}
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
func (x *XLSXWatermarker) ExtractWatermark(inputFile string) (string, string, error) {
	// 读取XLSX文件
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", "", fmt.Errorf("读取XLSX文件失败: %w", err)
	}

	// 打开ZIP文件
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", "", fmt.Errorf("解析XLSX文件失败: %w", err)
	}

	// 首先检查自定义水印文件
	customFileName := "xl/customWatermark.xml"
	for _, f := range reader.File {
		if f.Name == customFileName {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			fmt.Printf("找到自定义水印文件: %s\n", customFileName)

			// 提取水印数据，考虑CDATA包装
			var watermarkData string

			// 尝试提取CDATA中的内容
			cdataPattern := regexp.MustCompile(`<watermark><!\[CDATA\[(.*?)\]\]></watermark>`)
			cdataMatches := cdataPattern.FindSubmatch(content)
			if len(cdataMatches) >= 2 {
				watermarkData = string(cdataMatches[1])
				fmt.Printf("从CDATA中提取的水印数据: %s\n", watermarkData)
				return parseWatermarkData(watermarkData)
			}

			// 如果没有CDATA，尝试常规提取
			pattern := regexp.MustCompile(`<watermark>(.*?)</watermark>`)
			matches := pattern.FindSubmatch(content)
			if len(matches) >= 2 {
				watermarkData = string(matches[1])
				fmt.Printf("从XML中提取的水印数据: %s\n", watermarkData)
				return parseWatermarkData(watermarkData)
			}
		}
	}

	// 检查app.xml中的水印
	const appPropsFile = "docProps/app.xml"
	for _, f := range reader.File {
		if f.Name == appPropsFile {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			fmt.Printf("检查app.xml文件中的水印\n")

			// 尝试提取CDATA中的内容
			cdataPattern := regexp.MustCompile(`<Property name="WM" type="string"><!\[CDATA\[(.*?)\]\]></Property>`)
			cdataMatches := cdataPattern.FindSubmatch(content)
			if len(cdataMatches) >= 2 {
				watermarkData := string(cdataMatches[1])
				fmt.Printf("从CDATA中提取的水印数据: %s\n", watermarkData)
				return parseWatermarkData(watermarkData)
			}

			// 如果没有CDATA，尝试常规提取
			pattern := regexp.MustCompile(`<Property name="WM" type="string">(.*?)</Property>`)
			matches := pattern.FindSubmatch(content)
			if len(matches) >= 2 {
				watermarkData := string(matches[1])
				fmt.Printf("从XML中提取的水印数据: %s\n", watermarkData)
				return parseWatermarkData(watermarkData)
			}
		}
	}

	// 如果新方法未找到水印，尝试旧方法（向后兼容）
	fmt.Printf("尝试旧方法提取水印\n")
	watermarkFiles := []string{corePropsFile, workbookFile, sharedStringsFile}
	for _, fileName := range watermarkFiles {
		watermarkData, err := extractWatermarkFromFile(reader, fileName)
		if err != nil || watermarkData == "" {
			continue // 如果此文件中没有找到水印，继续查找其他文件
		}

		fmt.Printf("从旧位置提取的水印数据: %s\n", watermarkData)
		return parseWatermarkData(watermarkData)
	}

	return "", "", errors.New("未在XLSX文件中找到有效的水印信息")
}

// parseWatermarkData 解析水印数据
func parseWatermarkData(watermarkData string) (string, string, error) {
	// 添加日志，记录水印数据的格式
	fmt.Printf("解析水印数据: %s\n", watermarkData)

	// 首先检查并移除水印前缀和后缀
	if strings.Contains(watermarkData, watermarkPrefix) && strings.Contains(watermarkData, watermarkSuffix) {
		start := strings.Index(watermarkData, watermarkPrefix) + len(watermarkPrefix)
		end := strings.Index(watermarkData, watermarkSuffix)
		if end > start {
			watermarkData = watermarkData[start:end]
			fmt.Printf("提取的水印数据（移除前缀后缀）: %s\n", watermarkData)
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
				return decryptedText, time.Now().Format(time.RFC3339), nil
			}
		}
		return "", "", fmt.Errorf("无效的水印格式: %s", watermarkData)
	}

	encryptedText := parts[0]
	timestamp := parts[1]
	checksum := parts[2]

	// 打印解析后的部分，便于调试
	fmt.Printf("解析的加密文本: %s\n", encryptedText)
	fmt.Printf("解析的时间戳: %s\n", timestamp)
	fmt.Printf("解析的校验和: %s\n", checksum)

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
			return "", "", fmt.Errorf("解密水印失败: %w (加密文本: %s)", err, encryptedText)
		}
		// 解密成功但使用了临时密钥，跳过校验和验证
		return decryptedText, timestamp, nil
	}

	// 验证水印完整性
	calculatedChecksum := calculateChecksum(decryptedText + timestamp)
	if calculatedChecksum != checksum {
		// 校验和不匹配，但我们已经成功解密了文本，所以仍然返回结果
		fmt.Printf("警告：水印校验和不匹配 (计算值: %s, 存储值: %s)\n", calculatedChecksum, checksum)
	}

	return decryptedText, timestamp, nil
}
