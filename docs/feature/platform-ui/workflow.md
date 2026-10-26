# 平台化工作记录

## F0 / P0：仓库接入与 Workflow Gate Report

- 用户请求：Ant Design 前端平台化，新增登录、侧栏、主题、语言；直接提交 main。
- 分支：main（遵循用户明确要求，覆盖技能默认的 feature 分支规则）。
- 当前状态：初始工作区干净，远端 origin 为 GitHub 仓库；无 AGENTS.md。
- 阶段：P1 / F1 需求确认；任务类型为前端重构并新增服务端登录能力。
- 已有资料：README、Go 服务、原生 Web 页面、水印 service 测试。
- 所需上游资料：需求、页面状态、组件规范、前端架构、登录接口契约与验证计划。
- 缺失资料：无确认的需求文档、UX/设计及登录契约。
- 当前允许生产实现：否；先形成需求快照并确认身份与兼容性边界。
- 前置工作：需求文档见 requirements.md；确认后补齐架构及实施计划。
- 执行范围：Ant Design 工作台及原有水印操作页面、登录及鉴权、响应式导航、主题、中英双语、构建和部署说明。
- 验收：以 requirements.md 中 AC-001 至 AC-007 为准。
- 风险：已有 API 目前无需认证；增加登录会影响现有 API 客户端；不能以本地布尔值假冒登录。

## Feature Lifecycle Report

- 功能：水印工具升级平台。
- 用户场景：以统一工作台登录访问水印功能并设置显示偏好。
- 当前行为：原生单页，可匿名操作。
- 目标行为：Ant Design 平台导航、真实登录与可持续的主题语言偏好。
- 影响包：web、cmd/server、新增独立认证模块及前端构建入口（具体设计待需求确认）。
- 公共接口影响：预计保留水印路径与响应，但增加会话要求，待确认。
- 授权：用户要求直接提交 main；不创建 PR，不发布版本或部署。
- 阶段文档：本目录；后续每个阶段记录设计、实现及证据。
- 验证：Go 测试与鉴权集成测试、前端构建、浏览器添加/提取往返及偏好/导航验证。
- 文档：README、需求与设计、CHANGELOG。
- 提交计划：需求确认前仅提交文档；确认后设计、实现、验证按阶段提交并推送 main。
- 阻塞：需求快照待明确确认，特别是账号来源与 API 鉴权兼容性。

## Maintainability Gate Report

- 已检查：cmd/server/main.go、web/templates/index.html、web/static/js/app.js、web/static/css/style.css、internal/service。
- 触发：重构且跨越 UI、传输与后端；现有 CSS 为 1050 行。
- 风险等级：medium；原生页面耦合 DOM 与请求状态，后端入口混合装配、限流与业务路由。
- 责任边界：新认证逻辑应独立；前端应按布局、页面、API 和偏好状态拆分；不在原有大 CSS 上叠加新平台样式。
- 允许变更：boundary_redesign_plan；需求确认后形成模块拆分设计再实施。
- 验证标准：保留水印往返行为，鉴权在服务端生效，构建资源可由 Go 服务本地提供。
- 验证命令：git diff --check；实施后按实际构建入口补充命令与结果。

## F1–F3 / P1–P6：确认与设计完成

- 2026-10-26 用户明确回复“确认”，接受 requirements.md 中的默认方案。
- 需求确认提交：6f4d339；设计提交：56f57a1；实施计划提交：d852923；均已推送 main。
- 依赖状态：需求、界面状态、Ant Design 组件、前端架构、认证契约与验证计划均已记录。
- Workflow Gate：允许实现，范围以已确认需求为准，不需要额外产品决策。
- Maintainability Gate：按计划拆分认证与 React 页面；旧大 CSS 与 DOM 脚本退役，未在大文件中叠加行为。

## F4 / P7–P8：实现

- web/frontend 按 App、Shell、Login、Dashboard、WatermarkPage、Settings、Preferences、i18n、api 拆分；本地构建输出 web/dist，由 Go 服务提供。
- internal/auth 实现配置账号、随机令牌、摘要存储、会话探测、退出撤销、同源写请求检查与安全 Cookie 属性。
- cmd/server 将认证应用到原有水印路由；限流 map 加锁，限流只计登录/文件操作；上传请求体限长，multipart 临时文件回收。
- 保留原水印算法与 CLI；原生页面入口替换为 Ant Design 平台。
- 界面包括会话加载/失败、未配置、空结果、上传校验、处理中、成功下载/提取、错误重试与会话过期恢复。
- 偏好保存在浏览器；登录凭据不写 localStorage，退出清理页面状态。服务重启后会话失效。
- 验证：生产构建、Go 测试/race、认证 HTTP 测试、真实 Go 服务浏览器 PNG 往返已通过；最终验证记录见 verification.md。
