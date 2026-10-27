# C1：逐页PDF领域核心验收

仅验收核心库，不代表C目录任务/API/UI完成。整体目标继续。

## 实现与边界

`internal/pdfmark`按类型/选页/协议/解析提取/写入分离，实现pdf-page-v1页面对象HMAC标识。选页、跳过封面、逐页覆盖、源摘要与服务端时间进入标识。原文件只读，目标存在拒绝覆盖；新临时PDF完整重读并逐页核对后，用原子独占链接提交。关闭优化/全流解码，stateless/offline配置和对象/流/解码/递归/页数/输入/输出限制显式设置。

标识正文不加密；认证证明标识而不是任意视觉内容。搬移页面标识会保留签名但提示页位置/页数不一致；不宣称抵抗删除、同位置移植、重制或截图，也不把源摘要当作当前整页签名。未知key/坏签名不返回可信正文。已有签名/追加写限制拒绝改写；不主动扁平化表单。

## 可重复验证

- `go test -race ./internal/pdfmark`通过，含四份独立并发上下文；选页/封面/中文覆盖、真实最终解析、错key/未知key、篡改/搬移、现有标识/目的文件不覆盖、损坏/加密/取消、真实501页超限与文件/输出字节限制、声明签名文件拒绝及原件不变。
- `GOTOOLCHAIN=go1.26.0 go test ./internal/pdfmark`通过；同一最低版本下`go test -race ./...`全量通过，`go vet ./...`无诊断。`go mod verify`通过。通用解析器固定pdfcpu v0.16.3，要求Go1.26且许可证保存在third_party/pdfcpu；未引入blind_watermark或Python服务端依赖。
- 使用本仓库合成三页PDF，每次在副本上处理。三页Poppler渲染前后像素及解码内容完全一致，已打开检查三页；第二页保留交互字段，pypdf独立核对canonical `/Fields`值、有效widget值和非空`/AP /N`。独立Python HMAC实现核对最终对象的签名、页号、源摘要与每页正文，避免仅靠同一Go实现自证。
- [持久证据JSON](pdf-evidence/core-verification.json)与[保留表单的页面截图](screenshots/pdf-page-2-preserved.png)。测试原件/可再生成说明见internal/pdfmark/testdata；最终受保护测试PDF为临时QA产物，不是用户文件或产品交付包。

临时过程日志：`/tmp/workspace-c1-{tests-final,minimum-go,all-race,all-vet}.log`；复现PDF/渲染命令已保存在测试样例README。

## 下一步

C2接入有界ZIP/目录清单与逐文件处理、运行时/持久化、失败重试、通用资源审批桥接和真实API/UI，随后完整C验收。当前浏览器还没有逐页PDF入口，不能把此核心提交描述成PDF平台功能已全部交付。
