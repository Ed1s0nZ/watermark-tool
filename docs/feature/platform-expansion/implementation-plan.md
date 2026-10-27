# 实施计划与完整进度板

完整目标保留REQ-001～014；以下分批交付不是缩减范围。每批实现/验证记录单独提交，最终统一F5/F6验收。

| 批次 | 主要范围 | 状态 / 证据 |
|---|---|---|
| A | workspace加密事务、账号/成员、团队引擎隔离、每请求权限、前端团队选择 | 已完成基础切片：加密存储、版本认证、独立引擎、API/UI与13项浏览器/race/vet验收；见workflow A0～A2 |
| B | 接收人分组/导入选择、团队策略版本与审批执行门禁 | 契约已细化于b-collaboration-contract.md，待实施 |
| C | 混合目录/ZIP处理与失败重试、真实逐页PDF | 待实施 |
| D | 接收人受控链接、泄露调查证据/时间线/导出 | 待实施 |
| E | API Key、签名Webhook/outbox、Go SDK与示例 | 待实施 |
| F | 参数实测推荐、抗破坏专项、真实运维容量面板 | 待实施 |
| G | 全量权限/重启/异常/API/UI/视觉、README/CHANGELOG/回滚与最终审查 | 待实施 |

## A具体切片

- internal/workspace：types/store/validate/identity/teams与测试，职责分开；单元测试含密文无明文、错误key/篡改、重启、第二进程锁、写失败关闭写入、乐观版本冲突、跨团队拒绝和并发。
- internal/auth：注入可选用户provider/active checker，真实session身份、不改变旧构造函数，保留全套旧认证测试；补多身份与即时撤销。
- cmd/server/workspace*.go：初始化与有界JSON/handler/团队引擎适配，main只组装；独立资源释放/共享处理槽；新域未配置有明确503。
- web/frontend：共享workspace client/context、团队/成员页面与选择器、权限可见性，复用现有偏好/错误/上传；新增真实角色切换与数据隔离E2E。

## B–F文件与验证方向

每类能力在internal/workspace内领域模块与internal批处理/PDF/实验/通知模块分离，cmd/server加薄适配，前端每能力独立页及共享组件，不把所有渲染塞进Shell或一个通用JSON编辑器。SDK放pkg/client并提供examples，所有公开字段/错误/状态/限额在实施批次文档记录。

PDF/ZIP用真实有效fixture，不以伪PDF前缀验证解析。受控链接并发/过期/错密码/撤销/仅单接收人、审批改输入后失效、Webhook重定向/私网/签名/重试、SDK请求真实test server均有专门后端测试。运维/推荐必须从本次真实记录与处理结果推导。

## 检查与交付

每片 go test ./涉及包、race与必要负例；阶段后go test -race ./...、go vet ./...、前端TypeScript/Vite；新功能真实浏览器流程与全部旧12项回归。跨重启的用户/成员/策略/案件/任务/通知状态实际重开目录验证。截图打开审查中文桌面/英文暗色手机。新资源默认/留存/备份回滚在README说明。

最终所有REQ/AC证据成立才标目标complete；无外部部署、外部真实通知或版本发布。
