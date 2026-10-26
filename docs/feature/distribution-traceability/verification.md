# 批量副本、登记溯源与密钥版本：验证记录

日期：2026-10-26。范围：已确认 requirements.md 的 AC-001～AC-010。
实现提交：88f73bd；后续文档提交仅补充说明、截图与变更记录。

## 执行结果

| 检查 | 结果 / 证据 |
| --- | --- |
| `go test -race ./...` | 通过；包含存储域、认证、真实HTTP、盲水印固定夹具和现有服务回归。 |
| `go vet ./...` | 通过。 |
| 前端 `npm run build` | TypeScript检查及Vite生产构建通过。Ant Design包体积有非阻断警告，尚未按页面拆包。 |
| `npm run test:e2e` | 原有平台2个流程及新增分发流程1个，共3个通过。每套使用新服务保留真实默认限流。 |
| 直接 `npx playwright test distribution.spec.ts` | 单独运行新增流程也通过；终端设置无效部署密钥及另一部署目录时仍使用私有测试目录，避免测试覆盖真实数据。 |
| `node web/frontend/scripts/restart-smoke.mjs` | 真实编译进程通过：持久记录/密钥轮换、旧副本追溯、相同幂等ZIP、错误主密钥非零退出且持久文件不变、正常退出清理runtime、原图不变、日志无测试名单/主密钥。 |
| `GOOS=linux go test ./internal/distribution -c -o /tmp/watermark-distribution-linux.test` | Linux交叉编译通过；实际运行平台为macOS，未宣称Linux运行验证。 |

浏览器使用本机Chrome，命令前设置 `PLAYWRIGHT_CHROME_PATH` 指向其可执行文件。测试均为合成图片、测试账号和公开测试密钥，没有连接部署环境或外部发送服务。

## 验收映射

| 验收 | 验证证据 |
| --- | --- |
| AC-001 | `TestDistributionLifecycle` 与 `TestPlatformHTTP` 真实解开ZIP，逐份提取三名接收人的独立标识并匹配登记；检查原图字节不变。浏览器下载ZIP并提取Alice副本追溯。 |
| AC-002 | CSV空值/重复编号/超限/BOM/带引号值与输入校验；容量、像素及透明度沿用盲水印域测试；HTTP/浏览器重复编号提交明确失败。 |
| AC-003 | `TestConcurrentIdempotencyAndCSV` 并发同请求只接受一份处理；不同输入冲突。生命周期与真实重启测试验证重复请求ZIP一致。写失败会毒化存储并拒绝后续操作；取消删除部分包。 |
| AC-004 | 存储关闭重开、处理状态中断恢复、分页/查询边界、错误主密钥、损坏记录、缺失文件、进程锁及写失败测试。真实进程停止重启后记录仍可查。 |
| AC-005 | 验证后按副本ID、签名时间、算法及验证密钥共同关联；合法普通文本返回未登记，时间/密钥不匹配不关联。浏览器显示登记接收人及能力边界。 |
| AC-006 | `TestKeysAndMigration` / `TestPinnedKeysAndRegistrationBinding`；浏览器创建并切换版本，新批次检查新密钥ID，旧副本仍能验证，旧批次重生成ZIP摘要一致，停用旧版本。 |
| AC-007 | HTTP未登录401、同源保护及既有认证测试；密钥接口只返回元数据。加密文件不含原始密钥、主密钥不同于旧水印密钥、私有目录权限、日志不含测试名单/主密钥。 |
| AC-008 | 既有Go DCT v1固定夹具及兼容模式回归保留；旧环境密钥迁入管理库后原载荷可提取，单环境配置调用保持兼容。 |
| AC-009 | 每份副本从同一不可变解码原图生成，独立输出；并发槽与批次busy拒绝、128MiB包限制、取消清理、临时目录恢复清理测试。成功登记与最新重生成状态分离的专项回归通过。 |
| AC-010 | 上述构建、race/vet、真实API/浏览器/进程重启检查通过；README与日期分类CHANGELOG同步。最终提交与远端同步检查记于workflow.md。 |

实现测试位于 `internal/distribution/store_test.go`、`cmd/server/platform_test.go`、`web/frontend/tests/distribution.spec.ts`；重启冒烟位于 `web/frontend/scripts/restart-smoke.mjs`。

## 界面检查

已查看实际浏览器截图：桌面侧栏铺满视口，生成结果和登记信息完整；390px手机端英文界面无页面横向溢出，密钥表内部允许横向滚动，抽屉导航可切换。

- [批量生成截图](../../../image/distribution-batch.png)
- [登记溯源截图](../../../image/distribution-trace.png)
- [手机密钥管理截图](../../../image/distribution-keys-mobile.png)

## 验证边界

- 只生成和打包副本，不发送消息/邮件；成功登记不等于已经送达。
- 原图/ZIP临时处理，默认长期保留接收人登记元数据；数据目录及外部主密钥须成套备份。
- 登记接收人不等于实际泄露者；载荷验证不证明整张图片内容完整。
- 本轮没有升级水印算法的抗几何能力，不保证裁剪、缩放、旋转、截图或拍屏后的恢复。
- 单管理员、单写进程；不支持多实例共享目录、在线主密钥轮换、密钥明文导出或自动清理登记。
- 停用是服务层拒绝使用，保留密钥以便恢复，不是物理销毁。旧版v1载荷没有密钥编号，自动验证仅尝试未停用版本。
- 原有Office/PDF测试中依赖缺失外部样本的分支仍可能跳过，本轮未重新证明所有历史格式问题；实际新增闭环针对PNG/JPG图片。
