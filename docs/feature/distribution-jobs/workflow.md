# 多图分发任务与操作审计：阶段记录

## Workflow Gate Report / F0

- User request：新目标“最佳实践优化”，承接“还能加哪些能力”的候选讨论。
- Detected phase：P0–P1 / F0–F1，新增产品契约。
- Task type：多图后台任务、批量双水印及业务审计，不是已定位故障修复。
- Required upstream artifacts：确认的新范围、任务/文件生命周期、审计边界、恢复/验收标准。
- Found artifacts：distribution单图多人批次与登记/版本库、workbench单图双水印与实验/报告、单管理员认证、中英主题、真实API/E2E、已有README/CHANGELOG。
- Missing/weak artifacts：没有多图任务契约；没有下载有效期、任务恢复或审计一致性约定；上一轮推荐不等于本轮批准。
- Implementation allowed now：否；允许只读检查与需求草案。
- Prework required：形成可确认快照，显式标注新增数据留存及默认资源限制。
- Execution scope：建议多人多图任务、批量双水印、操作审计三项；其他候选不自动纳入。
- Acceptance criteria：见本目录requirements.md草案。
- Risks/assumptions：临时ZIP保留变化、原图保护、矩阵标识/重复提交、真实进度与取消、重启中断恢复、审计写入失败、单账号归属边界。

## Feature Lifecycle Report

- Feature：多图分发任务、按接收人双水印与操作审计（待确认）。
- Feature branch：main，沿用用户此前明确的直接main交付约定，覆盖技能默认独立分支要求。
- Repository hygiene：起始HEAD e21a050，与远端main一致；工作区干净，未找到适用AGENTS.md。
- Current phase：F0仓库检查完成；F1需求草案。
- User problem：整套素材按名单分发仍需重复单图操作，缺少后台进度/恢复；可见姓名与隐形登记尚未结合；只有HTTP路径日志，没有业务操作记录。
- Current / desired behavior：单图/请求内下载，单图双水印；期望为有界多图任务、接收人个性化双水印、可信范围明确的可查询审计。
- Affected packages：预计分发/任务持久域、密钥/认证事件适配、server、前端；确认后明确模块和迁移契约。
- Public surface impact：新增任务/审计接口和页面，保持原API/CLI/旧批次兼容。
- Safety / authorization impact：无外部发送或新增角色；临时文件保留、审计个人信息及密钥操作一致性须先明示。
- Required phase documents：requirements.md；确认后design.md / implementation-plan.md / verification.md。
- Required tests：多图多人真实提取映射、原图不变、幂等/重试/取消/重启/过期、密钥快照、失败状态、审计权限/秘密/故障、旧格式迁移、手机中英。
- Required docs / changelog：实现后README、根日期分类CHANGELOG、实际截图及逐项验证记录。
- Release impact：源码文档main交付，无线上部署或版本发布。
- Phase commit / push plan：F0检查文档、F1草案分别提交推送；确认后再设计、实现、验证。
- Blockers / assumptions：新目标范围及文件/审计政策尚未确认，不复用上一轮确认。

## F1：需求草案

- 16项要求、11项验收与7项假设明确列出；Status为Pending Confirmation。
- 特别区分生成状态与下载有效期、原图摘要与渲染摘要、登录账号与实际个人、服务端观察与实际送达。
- 5图/20人/100份、ZIP60分钟暂存、审计保留及写失败策略均为建议，未作为已授权产品政策实施。
- 需求/验收映射和git diff --check通过；当前仅文档，不运行无关生产测试。等待本轮快照确认后再进入F2。

## F1确认

用户回复“确认”，requirements.md已标Confirmed；允许设计、实现及验证，沿用main交付。

## F2–F3：设计与计划

- 需求确认add55e7、设计6ffcfc3、计划48d2ee6均已推送main。
- 采用v2同一规范快照解决关键业务/审计原子性；密钥字段单独AES-GCM加密，元数据整体HMAC，明确旧数据升级/备份/回滚限制。
- 任务输入与成品在runtime之外隔离保存；先完成持久域，再实现执行器、API及页面，范围无新增待决问题。

## F4切片1：原子存储及任务域

- 已实现v2规范快照、v1验证后迁移、同事务密钥/旧批次审计、查询与容量保留空间。
- 已实现多图矩阵任务/输入摘要/模板快照、幂等、队列/执行/取消/终态、重启恢复及Lookup/Records登记关联。
- 真实旧v1数据迁移后验证原水印/登记；模拟写前失败与已rename后返回错误，重启证明密钥和审计一起存在或一起不存在。
- 域及服务端race回归通过；新任务域测试覆盖并发同请求、两图三人实际嵌入提取关联、轮换、模板别名、渲染输入冲突与中断恢复。
- README已同步规范快照备份/回滚变化，旧测试改为检查真正的platform.json，原密钥/记录历史保持不改。
- 未完成：执行器/文件保留清理、HTTP/认证事件适配、任务与审计页面、完整端到端/重启过期/视觉验收。目标保持active，不宣称功能已交付。
