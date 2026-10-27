# C4：逐文件实际执行与最终验证

新增 `internal/fileprocess`，把安全接收层的服务端 Record 接入图片/PDF/Office 实际处理。此切片不是任务/API/UI交付，C仍进行中。

- 服务端随机物理路径与显示路径分开；开始前、预检后、提交前均检查实际字节数/SHA，拒绝链接/修改原件。参数固定文档ID/keyID/服务端秒级时间，输出目标独占创建，不覆盖原件或既有文件。
- 图片 PNG/JPEG 使用自研 Go DCT，真实34字节载荷/800万像素/不透明度/容量检查；默认保留PNG或JPEG（JPEG90），可显式改格式。对实际编码后文件重新解码/提取，质量门槛按最终文件像素PSNR；失败不交付。
- PDF调用真实对象核心，预检实际页数、选页/封面/逐页正文；最终报告逐页核验。标识认证不是视觉内容签名。
- Word/Excel/PPT/ODT验证内部包/CRC、必需主XML根与命名空间、XML结构/深度和签名声明；ODT检查Store mimetype和manifest。RTF检查实际嵌套、转义和binary跨度。兼容文件最终重新提取文字及固定添加时间，不称为密钥签发认证。
- 执行同步持有文件所有权，context取消不会让未跟踪goroutine继续写；临时目录0700/输出0600，全部验证和源摘要复核后fsync与独占link提交，失败清理。
- 输出扩展名冲突、Unicode规范/大小写冲突与文件-目录冲突统一在执行前分配确定后缀。保持相对父目录，ZIP调用层将成品放在files/下，与报告路径隔离。路径每段收紧为255字节、总512字节/16层，输出后缀UTF-8安全截断，无法容纳时显式拒绝。

## 证据

Go1.26 `go test -race ./internal/fileprocess ./internal/filebundle`通过；同包vet无诊断。测试使用完整应用库生成DOCX/PPTX/XLSX、合成ODT/RTF，始终复制原件再测；逐份最终提取/时间/SHA/字节一致，原件不变、正文/工作表/幻灯片字节一致。PDF实际三页跳封面/逐页覆盖后重新Inspect；PNG/JPEG真实提取/最终质量与60dB失败门槛/既有输出拒绝。变更摘要、取消、不支持类型、结构/签名、RTF转义/binary、输出重名和长中文名称均有负例。

独立Python按原包逐成员对比，最终输出只改变既有core.xml/meta.xml，DOCX段落/表格、PPT文字、XLSX原公式与值重新读取一致。ODT mimetype首项Store/无extra保持；不宣称Office桌面程序渲染。持久证据file-evidence/application-roundtrip.json，夹具及生成器位于internal/fileprocess/testdata。

临时日志 `/tmp/workspace-c4-{tests,final-race,final-vet}.log`；产物 `/tmp/watermark-file-qa`。
