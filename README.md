# 文档水印平台

基于 Go 的文档标记与图片盲水印平台，提供 Web 和 CLI。兼容模式处理 PDF、Office 等文件；自研 Go DCT v1 将带校验的短文本写入图片内容，可在明确的测试条件下经过重新编码后提取。不同模式的隐蔽性、抗处理和安全边界不同。

![Ant Design 平台工作台](image/platform-desktop.png)

## 变更记录

新增能力、Bug 修复及其他变更按日期记录在 [CHANGELOG.md](CHANGELOG.md)，包含迁入的历史更新时间线。

## 自研 Go 图片盲水印

水印引擎全部使用 Go 标准库实现，不依赖 `blind_watermark`、Python、OpenCV 或其他水印算法库。平台的添加/提取页面可选择：

- **文件标记（兼容模式）**：保留原有文件水印操作与旧文件提取。
- **图片盲水印 · Go DCT v1**：PNG/JPG/JPEG 图片中的亮度分块 DCT 嵌入，带版本、文本长度、时间戳和 HMAC 校验；提取无需原图或外部水印长度。

![Go 自研图片盲水印提取](image/go-blind-extract.png)

### 配置密钥

```bash
go build -o cli ./cmd/cli
# 只在首次建立密钥时生成；不要每次启动都生成新密钥。
export WATERMARK_BLIND_KEY="$(./cli blind-keygen)"
```

密钥为 32 个随机字节的 hex 表示（64 位十六进制），应通过安全配置保存并备份，同一份水印必须使用原密钥提取。v1 不提供密钥轮换库，换密钥后需保留旧密钥。密钥不进入网页或日志，不写入 Git。服务不会自动读取 `.env`。

未配置时只有新模式返回 `503 / blind_key`，兼容模式仍可用。服务器同时处理最多两个新模式任务，忙时返回 `429 / blind_busy`，可稍后重试。

### 使用与范围

```bash
# 输出路径必须不存在；拒绝覆盖原图、已有文件和指向原图的符号链接。
./cli blind-add source.jpg marked.png '项目-A100'
./cli blind-extract marked.png
# CLI 也可输出 JPEG（固定质量 95）
./cli blind-add source.jpg marked.jpg '项目-A100'
```

提取成功输出 JSON：`watermark`、UTC RFC3339 `timestamp`、`algorithm: "go-dct-v1"`。密钥错误通常表现为未检出；标记存在但版本/签名错误返回校验失败。HMAC 校验的是载荷真实性，**不加密文字，也不证明整张图片未被修改或水印无法移除**。

| 限制 | Go DCT v1 |
| --- | --- |
| 文本 | 非空有效 UTF-8，最多 34 字节，超限不截断 |
| 输入 | PNG/JPG/JPEG，最多 50 MB、800 万像素，不含透明像素 |
| 容量 | 至少 1536 个完整的 8×8 块；例如 320×320，或其他满足块数的长宽组合 |
| Web 输出 | PNG；新建文件，原图保持不变 |
| 元数据 | 按解码像素重新编码，不保留原图 EXIF、ICC 等元数据；需 EXIF 方向修正的图片应先规范化 |
| 几何处理 | 不承诺裁剪、缩放、旋转、截图或拍屏后恢复 |

### API

已登录的现有 multipart 接口新增可选 `algorithm`：缺省/`legacy` 保持旧行为；`go-dct-v1` 选择新模式；未知值返回 400。密钥仅由服务端环境提供。

```bash
curl -b cookies.txt -F 'algorithm=go-dct-v1' -F 'file=@source.jpg' \
  -F 'watermark=项目-A100' http://localhost:8080/api/add-watermark -o marked.png
curl -b cookies.txt -F 'algorithm=go-dct-v1' -F 'file=@marked.png' \
  'http://localhost:8080/api/extract-watermark?show_timestamp=true'
```

新模式返回 `X-Watermark-Algorithm: go-dct-v1`；提取 JSON 带 `algorithm`，时间戳按原 `show_timestamp` 参数控制。`422` 区分 `blind_not_found`、`blind_invalid`、`blind_capacity`、`blind_image`、`blind_text`；`503` 为密钥配置错误；操作仍受登录、同源和限流保护。

### 可复现的副本测试

```bash
go test -race ./...
go vet ./...
# 首次下载三张固定来源图片并核对预期 SHA-256；之后复用只读原图。
go run ./cmd/blind-bench -dir /tmp/watermark-go-bench
```

每轮建立独立 run 目录，每个压缩质量用例重新复制原图、嵌入，再从该水印输出派生压缩副本，不累积损伤。处理前后核对原图 SHA-256。报告记录样本来源、哈希、尺寸、完整载荷恢复、负例、PSNR 和含文件操作的耗时；基线或负例失败会返回非零退出码。测试使用固定公开测试密钥和固定载荷时间，不能用于生产。

当前三个样本（花朵、人像、咖啡静物）在 PNG 重新编码与 JPEG 95/85/75 下均恢复了载荷，无水印/错误密钥负例均拒绝；这只是样本结果，不是对任意图片的保证。画质改变和失败边界需结合自己的图片验证。见 [实测报告](docs/feature/go-blind-watermark/benchmark.md) 与 [验证记录](docs/feature/go-blind-watermark/verification.md)。下载图片和测试产物不入库；自制合成测试图与 v1 兼容夹具入库供离线回归。

## 平台界面与登录

前端使用 React + TypeScript + Ant Design，提供工作台、添加水印、提取水印和设置页面，支持响应式侧边导航、浅色/深色/跟随系统主题及简体中文/English。显示偏好保存在当前浏览器；不保存密码或登录令牌到 localStorage。工作台不存储处理历史或长期文件。

![登录界面](image/platform-login.png)

### 构建与启动（平台版）

需要 Go 1.24.4+、Node.js 20.19+ 或 22.12+、npm。首次启动前必须构建前端：

```bash
cd web/frontend
npm ci
npm run build
cd ../..
go build -o server ./cmd/server
export WATERMARK_ADMIN_USER=admin
# 请设置自己的密码，至少 12 字节；下面交互输入不会写入命令历史。
read -s WATERMARK_ADMIN_PASSWORD
export WATERMARK_ADMIN_PASSWORD
./server
```

从仓库根目录启动并访问 http://localhost:8080。服务提供本地 `web/dist` 资源，不依赖浏览器 CDN。每次修改前端后重新构建。`web/dist` 不提交到 Git，部署时需与二进制一起复制；无需运行 Node 服务。

| 环境变量 | 说明 |
| --- | --- |
| `WATERMARK_ADMIN_USER` | 必填，单管理员用户名 |
| `WATERMARK_ADMIN_PASSWORD` | 必填，至少 12 字节；不提供默认密码 |
| `WATERMARK_COOKIE_SECURE` | HTTPS 部署设 `true`；本地 HTTP 默认 `false` |
| `PORT` | 默认 `8080` |

配置缺失时登录返回 503，页面显示配置说明。`.env.example` 仅作变量参考，服务不会自动读取 `.env`。生产环境应通过 HTTPS 提供平台，并配置安全 Cookie。服务默认不信任代理提供的客户端 IP；代理部署需按实际网络配置可信代理后再按客户端 IP 限流。

会话使用随机令牌、HttpOnly / SameSite=Strict Cookie，最长 8 小时，退出即撤销，服务重启后失效。当前为单实例内存会话，不支持多副本共享登录。账号密码修改后需重启服务。不提供注册、密码找回、角色或用户管理。

### 开发与验证

```bash
# 终端 1：配置上述账号变量后，在仓库根目录运行
 go run ./cmd/server
# 终端 2：开发服务器（代理 /api 至本地 :8080）
 cd web/frontend
 npm run dev

# 仓库根目录运行 Go 检查
 go test -race ./...
# 前端目录运行
 npm run build
 npx playwright install chromium
 npm run test:e2e
```

E2E 使用专用本地 :18080 服务与测试账号，执行前需已构建前端，并保证端口空闲。测试覆盖登录、访问控制、文件往返、偏好保存和移动端导航。开发代理以 Vite 默认转发原始 Host 的方式保持同源检查。

### API 登录迁移（兼容性变化）

水印 API 路径、参数及成功响应不变，但现在必须先登录并携带会话 Cookie，匿名请求返回 401。CLI 使用不变。浏览器跨站写请求返回 403。登录与文件操作共享每 IP 每分钟 10 次限制，静态资源、会话探测、格式查询和退出不计入额度。

```bash
# 将凭据 JSON 写入本地受限权限文件，不将真实密码放进版本库。
curl -c cookies.txt -H 'Content-Type: application/json' \
  --data-binary @credentials.json http://localhost:8080/api/auth/login
curl -b cookies.txt http://localhost:8080/api/auth/session
curl -b cookies.txt -F 'file=@文档.pdf' -F 'watermark=内部资料' \
  http://localhost:8080/api/add-watermark -o watermarked.pdf
curl -b cookies.txt -F 'file=@watermarked.pdf' \
  'http://localhost:8080/api/extract-watermark?show_timestamp=true'
curl -b cookies.txt -X POST http://localhost:8080/api/auth/logout
```

`credentials.json` 格式为 `{"username":"你的账号","password":"你的密码"}`。Cookie jar 包含会话凭据，退出后删除本地临时文件。新增接口：`POST /api/auth/login`、`GET /api/auth/session`、`POST /api/auth/logout`。当前单文件最大 50 MB，水印文本最大 **100 个 UTF-8 字节**（并非 100 个中文字符）。

## 功能界面

以下为当前 Ant Design 平台的实际操作截图。

### 添加隐水印

![添加隐水印：上传文件、输入标识并下载结果](image/platform-add-watermark.png)

### 提取隐水印

![提取隐水印：查看水印内容与添加时间](image/platform-extract-watermark.png)

## 主要特性

- **两种处理模式**：文件隐藏标记与图片内容盲水印，按场景选择，效果以实测为准
- **载荷校验**：Go DCT v1 使用配置密钥验证水印载荷，兼容模式能力因格式而异
- **明确的提取结果**：新模式区分未检出、版本或签名无效、容量不足等情况
- **多格式支持**：兼容多种常用文件格式（PDF、DOCX、XLSX、PPTX、JPG、PNG等）
- **简洁界面**：直观易用的Web界面，操作简单快捷
- **API支持**：提供完整REST API，便于集成到现有系统
- **资源边界**：新模式限制文件体积、解码像素与并行任务数，超限明确拒绝

## 支持的文件格式

| 文件类型 | 水印添加 | 水印提取  | 备注 |
|---------|:-------:|:-------:|------|
| PDF     | ✅      | ✅      | 元数据隐写技术，文档外观无变化 |
| DOCX    | ✅      | ✅      | 在文档属性和内容中添加隐藏标记 |
| XLSX    | ✅      | ✅      | 在电子表格内部 XML 中添加标记 |
| PPTX    | ✅      | ✅      | 在幻灯片XML中添加不可见注释 |
| JPG     | ✅      | ✅      | 兼容模式使用 JPEG 注释；新模式写入图像内容 |
| PNG     | ✅      | ✅      | 兼容模式使用文件尾标记；新模式写入图像内容 |
| RTF     | ✅      | ✅      | 使用特殊字段隐藏水印信息 |
| ODT     | ✅      | ✅      | 在文档XML结构中添加隐藏标记 |

## 使用指南

### Web界面使用

1. 访问 http://localhost:8080，使用配置的管理员账号登录
2. 从侧边栏选择"添加隐水印"或"提取隐水印"功能
3. 上传文件并输入隐水印文本（添加水印时）
4. 等待处理完成后下载文件或查看提取结果

### 命令行(CLI)使用

除了Web界面外，本工具还提供命令行界面(CLI)，方便在脚本中使用或批处理文件。

#### 构建CLI工具

```bash
# 构建CLI工具
go build -o cli ./cmd/cli
```

构建完成后，CLI工具位于 `./cli`。

#### CLI命令

1. 添加水印

```bash
./cli add [输入文件] [输出文件] [水印文本]

# 示例
./cli add 文档.pdf 带水印.pdf "机密文件-请勿外传"
```

2. 提取水印

```bash
./cli extract [输入文件]

# 示例
./cli extract 带水印.pdf
```

3. 查看支持的文件类型

```bash
./cli types
```

### API接口使用

以下请求均需按前述迁移说明携带 `-b cookies.txt`，编程示例同样需要 Cookie jar 并预先登录。

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
curl -b cookies.txt -X POST -F "file=@文档.pdf" -F "watermark=机密文件" \
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
curl -b cookies.txt -X POST -F "file=@带水印文档.pdf" \
     http://localhost:8080/api/extract-watermark
```

#### 获取支持的文件类型

```http
GET /api/supported-types
```

示例响应：

```json
{
  "types": ["pdf", "docx", "xlsx", "pptx", "jpg", "png", "rtf", "odt"]
}
```

### 编程示例

以下是通过Go代码使用该工具的简单示例：

```go
package main

import (
	"bytes"
	"fmt"
	"encoding/json"
	"net/http/cookiejar"
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
	resp, err := httpClient.Post("http://localhost:8080/api/add-watermark", writer.FormDataContentType(), &buf)
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
	resp, err := httpClient.Post("http://localhost:8080/api/extract-watermark", writer.FormDataContentType(), &buf)
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

## 水印技术原理

兼容模式在文件属性、XML 或注释/文件尾等位置保存标记，不同格式的编码和保护方式不同，经过重新保存或元数据清理后可能丢失。

Go DCT v1 在 8×8 亮度块中投影两个正交 DCT 系数，通过系数差的正负嵌入比特。密钥派生的块顺序分散信息，重复编码并合并限幅证据恢复载荷；最后验证版本、长度和 HMAC。重复恢复用于抗噪声，HMAC 用于载荷校验，两者用途不同。具体协议见 [设计记录](docs/feature/go-blind-watermark/design.md)。

## 安全与能力边界

- Web 操作需要登录，生产环境应使用 HTTPS 与安全 Cookie。
- Go DCT v1 使用服务端配置密钥对载荷做 HMAC 校验；不提供载荷加密、整图完整性认证或不可移除保证。
- 兼容模式因格式而异：PNG/PDF 部分实现为 Base64 标记，JPG 使用旧版固定密钥。不能将所有兼容格式统一描述为强加密或防篡改。
- 旧格式时间信息按各处理器实现返回，不作为可信时间证明；自研模式的时间戳受载荷 HMAC 保护，但不等于第三方时间认证。
- 水印仅作为辅助标识手段，应结合访问控制、分发记录与具体业务流程使用。

## 注意事项

1. 图片内容盲水印会改变像素，使用前应评估画质与恢复率；不承诺不可检测
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
