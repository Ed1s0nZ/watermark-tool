# 隐形文档水印工具

一个安全、高效的隐形文档水印解决方案，可为各类文档添加完全不可见的数字水印，同时支持水印提取和验证。基于Go语言开发，支持web和cli的方式使用，水印采用隐写技术，确保水印完全隐形且不影响原始文档的阅读体验和视觉效果。

```bash
# 快速安装和启动
git clone https://github.com/Ed1s0nZ/watermark-tool.git
cd watermark-tool
go mod download
make build
./build/server
# 然后访问 http://localhost:8080
```


## Web展示
### 添加水印   
<img src="https://github.com/Ed1s0nZ/watermark-tool/blob/main/image/%E6%B7%BB%E5%8A%A0%E6%B0%B4%E5%8D%B0.png" width="800px">  

### 提取水印
<img src="https://github.com/Ed1s0nZ/watermark-tool/blob/main/image/%E6%8F%90%E5%8F%96%E6%B0%B4%E5%8D%B0.png" width="800px">  

## 主要特性

- **完全隐蔽**：水印完全隐形，对文档内容和外观零影响，肉眼无法识别
- **安全加密**：使用强加密算法确保水印信息安全不被篡改
- **防篡改设计**：独特校验机制确保水印不被非法修改，增强安全性
- **多格式支持**：兼容多种常用文件格式（PDF、DOCX、XLSX、PPTX、JPG、PNG等）
- **简洁界面**：直观易用的Web界面，操作简单快捷
- **API支持**：提供完整REST API，便于集成到现有系统
- **高性能处理**：优化的处理流程，快速处理各类文档，支持并发请求

## 支持的文件格式

| 文件类型 | 水印添加 | 水印提取  | 备注 |
|---------|:-------:|:-------:|------|
| PDF     | ✅      | ✅      | 元数据隐写技术，文档外观无变化 |
| DOCX    | ✅      | ✅      | 在文档属性和内容中添加隐藏标记 |
| XLSX    | ✅      | ✅      | 在电子表格内部XML中添加加密标记 |
| PPTX    | ✅      | ✅      | 在幻灯片XML中添加不可见注释 |
| JPG     | ✅      | ✅      | 使用EXIF注释和加密方式添加水印 |
| PNG     | ✅      | ✅      | 使用文本块添加隐藏水印 |
| RTF     | ✅      | ✅      | 使用特殊字段隐藏水印信息 |
| ODT     | ✅      | ✅      | 在文档XML结构中添加隐藏标记 |

## 快速开始

### 安装要求

- Go 1.18+（推荐使用Go 1.20或更高版本）
- 主要依赖：
  - github.com/gin-gonic/gin
  - github.com/gin-contrib/cors
  - github.com/google/uuid

### 安装步骤

1. 克隆仓库
   ```bash
   git clone https://github.com/Ed1s0nZ/watermark-tool.git
   cd watermark-tool
   ```

2. 安装依赖
   ```bash
   go mod download
   ```

3. 构建项目
   ```bash
   make build
   ```
   
4. 运行服务
   ```bash
   ./build/server
   ```

5. 访问Web界面
   ```
   http://localhost:8080
   ```

## 使用指南

### Web界面使用

1. 访问 http://localhost:8080
2. 选择"添加隐水印"或"提取隐水印"功能
3. 上传文件并输入隐水印文本（添加水印时）
4. 等待处理完成后下载文件或查看提取结果

### 命令行(CLI)使用

除了Web界面外，本工具还提供命令行界面(CLI)，方便在脚本中使用或批处理文件。

#### 构建CLI工具

```bash
# 构建CLI工具(同时也会构建Web服务器)
make build

# 或者只构建CLI工具
make ./build/watermark-cli
```

构建完成后，CLI工具位于 `./build/watermark-cli`。

#### CLI命令

1. 添加水印

```bash
./build/watermark-cli add [输入文件] [输出文件] [水印文本]

# 示例
./build/watermark-cli add 文档.pdf 带水印.pdf "机密文件-请勿外传"
```

2. 提取水印

```bash
./build/watermark-cli extract [输入文件]

# 示例
./build/watermark-cli extract 带水印.pdf
```

3. 查看支持的文件类型

```bash
./build/watermark-cli types
```

#### 使用Makefile快捷命令

项目提供了一些Makefile快捷命令方便使用：

```bash
# 运行CLI工具(显示帮助信息)
make run-cli

# 添加水印
make add-watermark 文档.pdf 带水印.pdf "机密文件-请勿外传"

# 提取水印
make extract-watermark 带水印.pdf

# 查看支持的文件类型
make types
```

### API接口使用

#### 添加隐水印

```http
POST /api/add-watermark
Content-Type: multipart/form-data

参数:
- file: 文件数据
- watermark: 隐水印文本
```

示例请求：

```bash
curl -X POST -F "file=@文档.pdf" -F "watermark=机密文件" \
     http://localhost:8080/api/add-watermark -o 带水印文档.pdf
```

#### 提取隐水印

```http
POST /api/extract-watermark
Content-Type: multipart/form-data

参数:
- file: 文件数据
```

示例请求：

```bash
curl -X POST -F "file=@带水印文档.pdf" \
     http://localhost:8080/api/extract-watermark
```

#### 获取支持的文件类型

```http
GET /api/supported-types
```

示例响应：

```json
{
  "supported_types": ["pdf", "docx", "xlsx", "pptx", "jpg", "png", "rtf", "odt"]
}
```

### 编程示例

以下是通过Go代码使用该工具的简单示例：

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// 添加隐水印示例
	filePath := "example.pdf"
	watermarkText := "机密文档-请勿外传"
	
	err := addWatermark(filePath, watermarkText)
	if err != nil {
		fmt.Printf("添加隐水印失败: %v\n", err)
		return
	}
	
	// 提取隐水印示例
	watermarkedFile := "example_watermarked.pdf"
	text, err := extractWatermark(watermarkedFile)
	if err != nil {
		fmt.Printf("提取隐水印失败: %v\n", err)
		return
	}
	
	fmt.Printf("提取的隐水印内容: %s\n", text)
}

// 添加隐水印
func addWatermark(filePath, watermarkText string) error {
	// 创建multipart表单
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// 添加文件
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}
	
	// 添加水印文本
	err = writer.WriteField("watermark", watermarkText)
	if err != nil {
		return err
	}
	
	writer.Close()
	
	// 发送请求
	resp, err := http.Post("http://localhost:8080/api/add-watermark", writer.FormDataContentType(), &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回错误: %s", resp.Status)
	}
	
	// 保存结果
	output, err := os.Create(filePath[:len(filePath)-4] + "_watermarked" + filepath.Ext(filePath))
	if err != nil {
		return err
	}
	defer output.Close()
	
	_, err = io.Copy(output, resp.Body)
	return err
}

// 提取隐水印
func extractWatermark(filePath string) (string, error) {
	// 创建multipart表单
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// 添加文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}
	
	writer.Close()
	
	// 发送请求
	resp, err := http.Post("http://localhost:8080/api/extract-watermark", writer.FormDataContentType(), &buf)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("服务器返回错误: %s", resp.Status)
	}
	
	// 读取结果
	var result struct {
		Watermark string `json:"watermark"`
		Error     string `json:"error"`
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	return string(body), nil
}
```

## 隐水印技术原理

本工具采用多种隐写技术实现不可见水印：

1. **元数据嵌入**：在文件元数据中嵌入加密水印信息
2. **内容隐写**：在文档内容的不可见部分嵌入编码信息
3. **结构隐藏**：利用文件格式结构特性隐藏水印数据
4. **加密保护**：使用AES-GCM等高强度加密算法保护水印内容
5. **校验机制**：添加校验和和时间戳确保水印完整性

## 安全特性

该工具采用多层次安全设计，确保水印信息的安全性：

1. **数据加密**：所有水印内容使用AES-256算法加密存储
2. **密钥管理**：系统使用安全的密钥生成和管理机制
3. **完整性验证**：使用校验和机制验证水印完整性，防止篡改
4. **时间戳**：每个水印都包含时间戳信息，可用于追踪溯源
5. **抗提取设计**：即使知道水印存在，没有正确工具和密钥也无法提取
6. **错误处理**：安全的错误处理机制，不会泄露敏感信息

为提高安全性，建议：
- 定期更换系统密钥
- 限制API访问权限
- 启用HTTPS加密传输
- 记录所有水印操作日志

## 注意事项

1. 虽然水印完全隐形，但在特定情况下可能被专业工具检测到
2. 水印安全性依赖于密钥的保密性，请妥善保管系统密钥
3. 不同文件格式的水印实现方式不同，安全级别可能有差异
4. 处理大型文件时可能需要调整服务器配置，增加超时时间

## 未来计划

- [ ] 支持更多文件格式（包括音频和视频格式）
- [ ] 添加批量处理功能
- [ ] 实现水印模板和权限管理
- [ ] 提供更高级的水印策略（如动态水印、分层水印）
- [ ] 开发桌面客户端版本

## 许可证

本项目采用 MIT 许可证 - 详见 LICENSE 文件 
