# 实现计划

- Feature：trace-policy-quality；Branch：main（用户既定约定）。
- 输入：Confirmed requirements.md；契约：design.md；Phase：F3。

| 切片 | 文件/模块 | 行为和证明 | 文档/回滚 |
| --- | --- | --- | --- |
| 1 | internal/quality、distribution jobs/types/validate/lifecycle、delivery archive | 纯RGB PSNR与策略，任务指纹及固定快照，实际逐份质量、失败整包；手算像素/边界、6份生成、失败/重试/旧库测试 | workflow记录；升级前备份，旧二进制不能保障新质量策略 |
| 2 | server trace-batch/workbench/jobs/audit/main | 受控128MiB多文件逐项流、真实Lookup、逐项错误与系统fatal、断连资源释放；dual原图对比和门槛，现有API兼容 | README新增API与状态；真实HTTP权限/资源/隐私测试 |
| 3 | 前端policy/quality/trace与Jobs/Dual/Shell/Reports | 共用策略控件与质量组件，中英/手机，导出安全、取消保留部分结果，本机存储明确失败 | 合成测试截图；旧模板不改动，不存载荷/名单 |
| 4 | Go/API/Playwright及restart smoke | 手算PSNR、门槛等值、真6份、坏图混合、取消、导出安全、存储损坏/满、旧流程/迁移回归 | verification逐项12REQ/8AC，README/日期CHANGELOG、F6审阅 |

生产依赖不变。所有读取和生成从独立副本执行，固定算法协议。改动文件按职责拆分，不将新功能塞入大文件；本次不是维护重构，现有待改模块均小于800行。共享控件避免页面复制。测试有真实数据流，不以mock假成功代替水印提取。

验证：go test -race ./...、go vet ./...、npm --prefix web/frontend run build、实际Chrome E2E、node web/frontend/scripts/restart-smoke.mjs。实际外观检查桌面/390px、中英/浅深、完整/取消状态，报告脱机脚本/公式安全。

每个完成阶段提交推送main。F4按最小贯通切片提交；F5文档/截图/证据，F6日期CHANGELOG与最终范围审阅、远端同步、工作区干净。仅源码交付，无部署、标签或外部发送。没有额外待确认决策。
