# 水印工作流优化：阶段记录

## Workflow Gate Report / F0

- User request：目标“最佳实践优化”；承接功能候选讨论。
- Detected phase：P0–P1 / F0–F1，新功能范围确认。
- Task type：平台功能与体验优化；不是已定位故障修复。
- Required upstream artifacts：已确认范围、用户场景、验收标准；确认后才需要交互/API/实现设计。
- Found artifacts：Ant Design中英主题平台；Go DCT v1及固定夹具；离线blind-bench；分发登记与密钥版本域；既有需求/设计/验证文档。
- Missing/weak artifacts：新目标未指定功能；上轮十项候选及优先三个仅为建议，不能视为全部获准。
- Implementation allowed now：否；允许只读检查、建立需求草案。
- Prework required：形成带假设的可确认需求快照。
- Execution scope：建议效果实验室、可见+隐形双水印模板、验证报告；待用户确认，不扩展到全部候选。
- Acceptance criteria：草案requirements.md；本阶段只核对可测试性及来源。
- Risks/assumptions：效果测试不等于增强算法；报告不等于第三方鉴定；中文可见文字需要一致渲染；文件留存/资源上限须明确。

## Feature Lifecycle Report

- Feature：水印实验、双水印和验证报告工作流（候选范围）。
- Feature branch：main，沿用用户此前明确的直接交付main约定，覆盖技能默认独立分支规则。
- Current phase：F0已检查，F1待确认。
- Repository hygiene：起始HEAD 7561137，与远端main一致，工作区干净；未发现适用AGENTS.md。
- User problem：目前难以在网页上检查图片经过处理后的可恢复性，缺少可见水印操作与可归档验证结果。
- Current behavior：Go盲水印单图/分发/验证、离线JPEG实验；没有上述三项网页闭环。
- Desired behavior：按已确认快照补齐评估、生成与报告工作流。
- Affected packages：预计blind/实验处理域、server、前端；边界确认后在设计中定义。
- Public surface impact：预计新增入口及受认证API；不改动v1协议、既有分发记录或密钥语义。
- Safety / authorization：仅本地副本处理和用户下载，无外部发送；是否持久化模板明确列为待确认假设。
- Required phase document：本目录requirements.md，确认后design.md、implementation-plan.md、verification.md。
- Required tests：原图不变、真实处理/提取、失败行与负例、双水印实际提取、中英/主题/移动端、报告与实际结果一致。
- Required docs / changelog：实现后README及根日期分类CHANGELOG。
- Release impact：直接main代码交付；不自动部署或发布版本。
- Phase commit / push plan：F0记录、F1需求草案分别提交推送；确认后再推进设计/实现。
- Blockers / assumptions：当前仅缺新范围的明确确认；不把上一轮确认用于本轮新增需求。

## F1：需求快照

- F0提交c0b54b0已推送main。
- requirements.md状态Pending Confirmation；14项要求、8项验收，范围建议及6项假设均明示。
- 质量检查：每项验收有需求映射；实验能力与算法增强分开；报告真实性、模板留存、中文渲染、临时文件、兼容路径及失败恢复已覆盖。
- git diff --check通过；仅文档变更，不运行无关生产测试。
- 等待用户确认本轮快照后继续F2；当前目标保持active，不宣称已完成功能优化。

## F1确认

用户回复“确认”，需求快照已改为Confirmed。允许设计与实现，沿用main交付。
