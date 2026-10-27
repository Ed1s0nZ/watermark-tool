# Logo、区域避让及JPEG导出：阶段记录

## Workflow Gate Report / F0

- User request：用户询问还能做哪些，助手建议优先Logo水印＋区域避让＋JPEG导出；新目标“最佳实践落地”。
- Detected phase：P0–P1 / F0–F1，新一轮产品契约。
- Task type：扩展可见层编辑、实际输出格式和最终文件验证。
- Required upstream artifacts：明确Logo/避让范围、输入上限与留存、最终JPEG验证/质量语义、任务恢复和兼容规则。
- Found artifacts：Canvas文字预览、单图Go与双水印、多图任务/固定快照、质量门槛、本机策略、流式溯源和完整回归。
- Missing/weak artifacts：现有仅文字可见层、无逐图避让坐标、Web仅PNG成品；JPEG实验不等于正式导出。缺少上述三项的已确认契约。
- Implementation allowed now：否；可以整理需求确认包，不将建议的三项或全部九项候选自动视为已确认。
- Prework required：requirements.md提出保守默认并明确假设，等待该快照确认。
- Execution scope：建议Logo、手动矩形避让、PNG/JPEG可选输出及对应恢复/质量整合；其他候选不自动纳入。
- Acceptance criteria：见本目录requirements.md。
- Risks/assumptions：可见层避让不代表隐形层像素不变；JPEG最终文件必须复验，失败不能用PNG或调质量冒充用户选项；Logo和矩形不能变成永久素材/个人信息库。

## Feature Lifecycle Report

- Feature：Logo、区域避让与验证后的JPEG导出（建议范围）。
- Branch：main，沿用用户此前直接main交付约定，覆盖技能默认独立分支要求。
- Hygiene：起始HEAD 5427e04，origin/main一致，工作区干净；仓库未发现AGENTS.md，祖先指令此前核对无新增已知指令。
- Current phase：F0完成，F1需求准备；不复用前一轮的确认。
- Current behavior：visibleWatermark.ts仅渲染文字；任务Style严格六个字段；单图Go/双水印/任务ZIP均输出PNG；quality区分嵌入与可见层，未度量JPEG交付后变化。
- Desired behavior：品牌图片可见层、逐图避让、用户选定JPEG后基于实际成品验证/门槛/摘要。
- Affected packages：预计前端共享编辑/策略/预览，Go编码与质量、单图及后台执行器、任务持久域和报告；确认后确定边界。
- Public surface impact：新增可选可见层和输出选项，任务固定快照/清单；默认PNG与旧指纹保持兼容。
- Safety / authorization：不外发、不永久保存Logo；新输入默认上限及最终JPEG质量语义待确认。
- Required phase documents：requirements.md；确认后design.md / implementation-plan.md / verification.md；最终README/日期CHANGELOG。
- Required tests：独立原图/Logo不变、透明Logo合成、避让预览像素、逐图坐标/缩放/手机、最终JPEG解码真实提取/门槛、失败整包、重试摘要、旧格式/策略/指纹及全部回归。
- Commit / push：F0记录单独提交推送；F1确认稿单独提交推送；确认后进入设计实现。
- Release impact：源码文档交付，不部署/打标签、不引入外部水印库。

## F0检查结论

已核对当前工作区和渲染/编码/质量代码。现有JPEG实验仅测试恢复，不可直接当成正式成品；PNG预编码像素校验亦不能替代最终JPEG复验。本轮需要新产品契约，尚未修改生产代码。

## F1需求确认稿

- requirements.md为Pending Confirmation：12项要求、8项验收、5项建议假设，来源与新政策均明示。
- 核心区别：预览像素避让只属于可见层；最终JPEG重新解码/真实提取并应用门槛，不能拿编码前结果保证交付。
- Logo临时使用、逐图矩形不存本机策略、旧默认指纹及旧策略兼容均纳入验收；没有扩展全部候选。
- 当前仅文档，轻量检查REQ/AC映射与git diff --check；不运行无关生产测试。按requirements-doc-write明确确认门槛等待该快照确认，未进入设计/代码实现，不宣称本轮已完成。
