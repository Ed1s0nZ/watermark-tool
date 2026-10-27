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
