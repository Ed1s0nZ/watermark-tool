# 平台化验证记录

- 日期：2026-10-26（Asia/Shanghai）
- 功能提交：92439ae（main，已推送）
- 验证对象：本地 Go 服务真实提供的 Vite 生产构建；非 mock 页面。

## 命令与结果

| 检查 | 结果 |
| --- | --- |
| npm run build --prefix web/frontend | TypeScript 与 Vite 成功；依赖已锁定，运行时资源在本地 |
| go test -race ./... | 通过，包括认证生命周期/并发会话、限流并发测试 |
| go vet ./... | 通过 |
| PLAYWRIGHT_CHROME_PATH='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' npm run test:e2e --prefix web/frontend | 最终 1 passed（10.8s），一个完整跨页面流程，包含下表断言 |
| git diff --check | 通过 |

测试环境已安装 Chrome，故使用可选 PLAYWRIGHT_CHROME_PATH。其他环境按 README 安装 Playwright Chromium 即可，配置默认使用其 bundled browser。初始运行遇到浏览器版本未安装、装饰图标的 accessible name、Select 输入被显示文本覆盖、测试下载文件缺扩展名及过渡动画问题；已修复装饰图标可访问性，并以有状态的键盘选择/明确下载文件名/可观察的动画结束条件稳定测试。最终无 pageerror。

## 验收映射

| 验收 | 当前证据 | 结论 |
| --- | --- | --- |
| AC-001 Ant Design、侧栏与响应式 | 实际组件导入；工作台入口；折叠/展开；390px Drawer 导航与页面无溢出断言；桌面/手机截图检查 | 完成 |
| AC-002 登录鉴权 | 错密码拒绝、正确密码登录、刷新恢复；匿名水印操作/格式查询 401；退出撤销；模拟服务端退出后真实请求 401，引导重新登录并保留草稿；单元测试 Cookie 属性/轮换/8小时过期/缺配置/跨源403 | 完成 |
| AC-003 主题 | UI 选择深色并刷新保持；选择 System 后模拟系统 light/dark 均跟随；登录与平台共用偏好组件 | 完成 |
| AC-004 语言 | UI 中文切 English；切换保留输入；刷新后 html lang=en、页面英语；类型化词典覆盖所有业务文案，Ant Design locale 随动 | 完成 |
| AC-005 文件行为与校验 | 支持类型精确为八种；真实 PNG 添加→下载→重新上传→提取文本一致；带/不带时间戳分别验证；无文件、非法格式、50 MB+1、102字节文本被拒绝 | 完成 |
| AC-006 加载和恢复 | 真实请求前延迟验证处理中状态；网络中断后保留输入且重试成功；同步 pending 防重复提交；服务端拒绝未登录操作 | 完成 |
| AC-007 构建、测试与提交 | 上述构建/Go/race/vet/浏览器通过；功能代码已推送 main；文档与最终同步状态随收尾提交核对 | 完成，最终 Git 状态由提交后的检查确认 |

## 视觉证据

- [工作台](../../../image/platform-desktop.png)
- [登录页](../../../image/platform-login.png)
- [390px 手机设置](../../../image/platform-mobile.png)

截图用于产品文档，显式纳入版本库；测试运行目录、50 MB 测试文件、node_modules、dist、Cookie、账号私密配置均不入库。

## 已知边界

- Vite 提示 Ant Design chunk 约 730 KB（gzip 226 KB）超过默认 500 KB 警戒线；构建成功，依赖与应用代码已分包。后续可按页面进一步懒加载。
- 原有 service 测试中四种 Office/PDF 样本未提供，相关子测试按原逻辑跳过。本次实际文件往返验证为 PNG，其余格式的算法未修改；不声称已逐个格式做真实文档回归。
- 单管理员、单进程内存会话为已确认范围；不含多用户、SSO、持久历史、共享会话或外部部署。
- 前端产物需要部署前构建；旧 API 客户端必须迁移到会话 Cookie；CLI 原有入口不变。


## 布局反馈回归

- 用户反馈：首版营销横幅、静态参数统计卡与重复入口导致布局不自然；侧栏悬空。
- 调整：简短页头 + 两个水印操作入口 + 单组格式/限制说明；64px 顶栏与品牌区域对齐；应用固定为视口高度，只有内容区滚动，侧栏满高，底部操作贴底。深色容器 token 统一。
- 最终前端生产构建成功；更新后的真实服务 Playwright 1 passed（12.4s）。
- 新增明确几何断言：1280×720 和 1280×900 时侧栏 top=0、bottom=innerHeight；长水印页面滚动内容后仍成立，顶栏仍在视口内。
- 原登录、下载/提取往返、校验、主题/语言、手机导航、会话失效恢复流程全部继续通过。
- 已重新检查并更新桌面、手机截图；README 展示当前布局。后端代码未变化，沿用本次已通过的 Go race/vet 结果。
