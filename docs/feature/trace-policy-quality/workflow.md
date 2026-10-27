# 批量溯源、策略预设与质量检查：阶段记录

## Workflow Gate Report / F0

- User request：用户新目标“最佳实践完成”，承接上一轮新增能力建议；建议优先项为批量溯源中心、水印策略预设、自动质量检查。
- Detected phase：P0–P1 / F0–F1，新一轮产品契约，不复用上一轮多图任务的确认。
- Task type：扩展已有追溯、模板和生成验证能力。
- Required upstream artifacts：新范围、批量限制、策略保存边界、质量指标含义及验收标准。
- Found artifacts：现有单图溯源/报告、浏览器样式模板、实验室PSNR、多图任务及真实逐份验证、v2原子审计。
- Missing/weak artifacts：没有批量溯源契约，没有模式＋文字＋质量策略组合契约，也没有生产结果质量门槛的含义。
- Implementation allowed now：否；允许形成需求确认包，不能把推荐的三项或十项候选自动视为已确认产品契约。
- Prework required：requirements.md明确三项建议范围及默认值，等待该快照明确确认。
- Execution scope：建议仅三项；团队、API密钥、相似检索、遮罩、PDF与新几何算法不自动纳入。
- Acceptance criteria：见requirements.md的AC-001～008。
- Risks/assumptions：登记接收人不是实际泄露者；PSNR不是人眼质量认证或抗攻击保证；质量相对何种输入必须明确；策略不能长期保存图片/名单/密钥。

## Feature Lifecycle Report

- Feature：批量溯源中心、可复用策略、生成质量检查（建议范围）。
- Branch：main，沿用用户此前直接main交付约定，覆盖技能默认独立分支。
- Hygiene：起始HEAD 0f3834e，origin/main一致，工作区干净；本仓库与祖先路径未找到AGENTS.md。
- Current phase：F0完成，F1需求准备；此前多图任务目标已经完成，新目标不可据此跳过确认。
- User problem：可疑图片要逐一验证；现有模板只保存样式；生成成功缺少统一的量化质量展示。
- Desired behavior：批量真实验证并汇总、配置可复用、明确质量指标与失败结果。
- Affected packages：预计server/workbench/distribution/delivery、共享前端策略/报告/质量展示及测试；确认后设计边界。
- Public surface impact：新增批量流程、可选策略和质量字段；已有Go v1/legacy/CLI及旧批次行为应保留。
- Safety / authorization：受现有认证和审计保护，无外部发送；新增默认阈值和本机保存规则待确认。
- Required documents：requirements.md；确认后design.md、implementation-plan.md、verification.md；最终README/CHANGELOG。
- Required tests：真实多图注册/未登记/未检出混合结果、逐项失败/取消、原图不变、策略刷新/损坏/迁移、PSNR参考及阈值边界、旧流程与手机中英。
- Phase commit / push plan：F0单独提交推送，F1草案单独提交推送；确认后继续设计、实现与验收。
- Release impact：源码/文档交付，不部署、不创建标签；不新增水印算法依赖。

## F0检查结论

- `TraceForm.tsx`与`/api/workbench/verify`现为单图；已有JSON/HTML报告可复用。
- `visibleWatermark.ts`的Template仅含name/style，本机最多20个，不保存文字/模式。
- `internal/workbench/lab.go`已有PSNR，但只用于实验；`internal/delivery/archive.go`已逐份提取校验，不将它重述为新功能。
- 只读检查改变了范围判断：本轮强调批量汇总、组合预设和生产质量度量；没有修改生产代码。

## F1：需求确认包

- requirements.md为Pending Confirmation，12项要求、8项验收、5项待确认假设；每项建议来源与已有约定分开标识。
- 原建议“质量检查”细化为默认展示、可选PSNR门槛，并明确双水印参考图，避免把已有提取验证或低PSNR直接解释为产品缺陷。
- 当前只提交需求文档，未修改生产代码。轻量检查包括本地链接、需求验收映射和git diff --check；尚未进入代码测试阶段。
- 按requirements-doc-write的明确确认门槛暂停设计/实现，等待用户确认此快照；目标保持active，不宣称本轮功能已完成。

## F1明确确认

2026-10-27用户回复“确认”，需求标记Confirmed；允许继续设计、实现、验证与main交付。此前确认阻塞解除。

## F2–F3

- 使用technical-plan-write，从已确认需求完成design.md与implementation-plan.md；不新增产品范围。
- 批量以受限请求流式返回逐项结果；策略留在本机；质量计算独立纯Go模块、服务端计算并与既有任务快照/原子审计衔接。
- v2可选字段兼容旧数据；门槛不改变算法。新策略任务回滚旧二进制不能保证门槛，须恢复升级前备份。
- 依次提交推送设计/计划后进入实现；不要求用户再次批准已确认范围。

## F4进行中：贯通实现与首次验证

- 已实现纯Go质量度量/门槛，任务策略指纹、逐份质量与失败整包，独立参考的双水印质量；旧请求省略策略时保持原指纹。
- 新增批量溯源受限NDJSON接口，真实逐项验证/Lookup、重复摘要、单项坏图、系统fatal和断连清理；审计仅关联随机批次。
- 前端共用策略CRUD/明确应用、质量设置与摘要，新增临时批量报告/取消/安全JSON/CSV/HTML。
- 手算PSNR/阈值边界、真实2图3人质量、60dB失败整包/重试/跨重启、旧v2无质量字段/原指纹、HTTP混合结果/取消/权限/隐私已通过聚焦race；全量Go race/vet通过。
- 浏览器首次运行发现的是测试断言与Ant Design精度格式、已被取消的路由处理器竞态，不是产品验证伪成功；正在修正测试并继续实际完整流程验收。此阶段尚未宣称F4/F5完成。

## F4贯通验证完成

- 新增浏览器策略CRUD/刷新/20项上限/损坏和QuotaExceeded存储、真实混合批量流、JSON/CSV/HTML安全、取消保留未确认项、单图60dB拒绝/默认通过、手机英文深色均已实跑通过。
- 未收到结果的项目明确为not_confirmed，不因取消或断流断言服务器绝未执行，更不当作阴性。
- 新增功能未改Go/npm生产依赖清单。接下来F5跑新增及全部旧浏览器流程、重启冒烟、视觉检查和文档；不自动部署。

## F4补充：兼容与可访问性收尾

- 默认省略策略和显式空策略采用相同旧指纹，既有幂等请求不因默认序列化形式变化而冲突；非默认阈值仍参与指纹。旧v2任务无指标保持未记录。
- 批量文件数量/单份/总量限制提取为共享校验并测试精确边界；实际编码头超800万像素被解码前拒绝，忙槽返回429并保持资源可复用。
- 全量浏览器流程暴露快速流结束时加载图标改变按钮可访问名称的竞态，开始按钮增加固定aria-label，保持读屏/自动化名称稳定；重新跑完整流程。
