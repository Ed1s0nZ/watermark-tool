# 实现计划

Branch：main（用户既定约定）。输入：已确认requirements.md；契约：design.md；阶段F3。

| 切片 | 模块 | 行为及检查 |
| --- | --- | --- |
| 1 | internal/output、quality、composition、distribution | 标准库编码/最终解码验证、指标/门槛、艺术层快照与默认指纹兼容；手算指标、文件签名、真实JPEG正负例、范围/旧库/指纹测试 |
| 2 | server单图/dual/jobs、delivery | 接入最终文件响应和逐份ZIP，输出/艺术层严格校验，错误摘要、完整包失败；真实HTTP/六份/恢复/权限/审计测试 |
| 3 | 浏览器共享composition/render/editor/output | Logo透明合成、五位置避让、归一化框选/数值、逐图快照、单图Go/dual/jobs导出、旧策略可选扩展；实际像素/手机交互/存储测试 |
| 4 | 全量QA与文档 | 真实成品逐份提取、阈值编码前后差异、取消/重试/重启/过期及旧流程；README/日期CHANGELOG/4类实际截图/verification |

各模块按职责拆分，不扩展旧Style六字段，不新增生产依赖。不得用模拟验证通过代替实际下载解码/提取，不自动提高JPEG质量或回退PNG。原图和Logo均从副本处理。

验证命令：go test -race ./...、go vet ./...、npm --prefix web/frontend run build、PLAYWRIGHT_CHROME_PATH指定本机Chrome运行完整test:e2e、node web/frontend/scripts/restart-smoke.mjs；聚焦用例用于迭代，最终证据覆盖全部12REQ/8AC。

每个完成阶段提交推送，F4按连贯切片提交，F5使用说明/证据/截图，F6变更记录与最终远端同步。格式字段可选、旧v2/指纹/策略兼容；新任务回滚旧二进制不能保证输出/艺术层，须恢复升级前完整备份。不部署、不打标签、不对外发送。无新增待确认问题。
