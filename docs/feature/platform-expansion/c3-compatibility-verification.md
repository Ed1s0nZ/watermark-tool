# C3：混合目录前置的兼容格式修复

目录任务将调用真实处理器。验证中发现原 Word/PPT 插入原文会破坏 XML/comment 或截断空格，Excel 新 custom.xml 属性不完整，ODT 重打包破坏首位 Store mimetype，RTF/旧 ODT AES 密钥37字节非法，RTF 提取正则及输出 NUL 填充错误。先修正现有入口，再接任务运行时。

## 结果和边界

- `WatermarkDataV2` 注释为有版本 JSON/Base64，完整 UTF-8 正文及真实添加时间；识别限定于根内保留注释、严格元数据，更新去除同协议旧注释。按原 XML 字节插入，不重新序列化正文/命名空间。
- Word/Excel/PPT 优先既有 core.xml，ODT 优先 meta.xml，否则回退既有主 XML；其余 ZIP 成员以原始压缩内容复制，ODT mimetype 保持首位、Store、无额外字段。不上新增无声明的属性或元数据文件。
- 有界内部 ZIP/CRC/路径预检；已声明 OOXML/ODF 数字签名结构拒绝改写，不冒充密码学签名验证。输出私有临时写入、fsync、最终重读正文/时间后提交。旧 CLI/API 输出覆盖行为保持；新目录执行器另提供独占非覆盖输出。
- 新元数据不加密也无签发者认证，不能证明正文未修改。RTF 固定公开口令 SHA-256 派生修正非法 AES 密钥，只是兼容编码。原37字节口令不能生成有效 AES 输出，故不虚构该错误实现的兼容夹具。
- 旧文件读取路径保留；Word/PPT/Excel 旧记录缺时间返回空，不捏造当前时间。Excel 旧 CFB 仅恢复数据，不宣称认证；不再打印密文/正文。旧二进制不认识新 V2 输出，升级回滚须保持完整数据备份。

## 验证

Go1.26 `go test -race ./internal/service ./internal/watermark/...`、`go test -race ./cmd/server` 和所涉包 vet 通过（服务端90.927s）。专项覆盖实际 ZIP 数据流：A&B、双连字符、中文、标签/引号/空格、固定2002时间与实际公开入口添加时间、原件不变/主 XML 字节不变、旧时间未记录、穿越拒绝、声明签名拒绝。RTF 含 info 与无 info 两种、完整恢复及无 NUL。ODT 结构夹具验证 mimetype 首项 Store/无 extra、manifest/content 原字节和包成员数不变。XML 单元覆盖 BOM/前缀/自闭合/更新/恶意结构。

以上 Office 夹具用于包/XML兼容性测试，不宣称 Microsoft Office/LibreOffice 实机渲染验收。完整应用生成样例的逐文件最终验证在 C4 继续。HTTP 原平台两个真实浏览器用例通过（29.1s）；C 的任务/API/UI仍待完成。

临时日志：`/tmp/workspace-c3-{last-race,server-final,vet-final,browser-final}.log`。
