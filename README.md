# 文档水印平台

基于 Go 的文档标记与图片盲水印平台，提供 Web 和 CLI。兼容模式处理 PDF、Office 等文件；自研 Go DCT v1 将带校验的短文本写入图片内容，可在明确的测试条件下经过重新编码后提取。不同模式的隐蔽性、抗处理和安全边界不同。

![Ant Design 平台工作台](image/platform-desktop.png)

## 变更记录

新增能力、Bug 修复及其他变更按日期记录在 [CHANGELOG.md](CHANGELOG.md)，包含迁入的历史更新时间线。

## Logo、区域避让与JPEG导出

单图“双水印模板”和“多图分发任务”的双水印模式新增 **仅文字／仅Logo／文字＋Logo**。Logo使用PNG/JPG，最多5MiB、200万像素，可有透明背景；原图仍须满足50MiB、800万像素、非透明及水印容量限制。Logo等比缩放，支持五个位置、宽度占原图5%～40%（默认15%）及不透明度0.05～1（默认0.35），默认右下。仅Logo不要求可见文字，隐形载荷/每人独立标识仍按原规则生成。

### 逐图避让与预览

选择原图后，在“可见水印避让区域”用鼠标/手指框选，或编辑x/y/width/height百分比；每图最多8个，可删除、清空。矩形按原始文件摘要绑定，缩放不改变实际区域，切换不同图不会套用其他图片的坐标。

单处水印先尝试所选位置，冲突后尝试其他既有位置；平铺文字跳过与避让区相交的完整实例，不截断文字。启用的文字或Logo如果都没有完整可用位置，会明确拒绝预览，需缩小水印或调整区域。原图不被裁剪、模糊或擦除。

**区域只约束可见层，隐形嵌入和JPEG编码仍可能改变区域内像素。** 图像预览与矩形编辑分别展示，最终成品还包含这些像素变化。逐张图/逐个接收人核对真实预览；改变图、Logo、文字、布局或区域后须重新预览。客户端提交同一渲染PNG，不在服务器重新排版字体，也不宣称识别认证了图中Logo。

### PNG／JPEG及最终文件检查

“添加水印”的Go模式、单图双水印和多图任务均可选PNG/JPEG，默认PNG。JPEG质量60～100、默认90，单个任务统一格式和质量。程序使用Go标准库编码，再对 **最终完整文件** 重新解码、真实提取并核对载荷/时间/版本后才交付；失败不会自动调高质量、改变算法或回退PNG。压缩不保证所有图片均能恢复，错误会明确展示。

质量摘要增加：

- 隐形嵌入变化：实际嵌入输入与嵌入后像素。
- 编码变化：嵌入后像素与最终文件解码像素。
- 最终成品变化：实际嵌入输入与最终文件解码像素，**可选PSNR门槛使用此项**；不能拿JPEG编码前通过结果冒充成品通过。
- 双水印另外保留可见层相对原图的变化。各项独立计算，不相加；PSNR不能证明人眼质量或抗裁剪/拍屏能力。

摘要还包含实际格式、JPEG质量、字节数和最终文件SHA-256；PNG的编码变化通常为∞，旧记录没有新字段时显示未记录。阈值使用未舍入值，默认只展示；不合格份有真实失败摘要，任务整包失败且不开放部分下载，不撤销其他任务已完成登记。ZIP中的png/jpg路径、实际字节和manifest.csv质量JSON对应。

### 策略、恢复与留存

本机策略可保存可见层组合、Logo布局标量及输出格式/质量；**不保存Logo文件/摘要、原图或逐图矩形**。旧v1策略和旧六字段样式模板继续可用。应用策略或载入任务参数后需重新选择Logo，文件框会清空；恢复时核对同一Logo摘要和原图/渲染摘要。改Logo、区域、格式/质量或门槛不能冒充原任务重试，应明确创建新任务。

任务固定必要的Logo身份、每图矩形、渲染身份和输出选项，跨中断保持原ID、时间和密钥。Logo原始文件只在当前浏览器临时解码，计入512MiB预览输入预算；服务器只接收有界原图/渲染PNG并校验声明快照及摘要，不保存Logo素材库。现有15分钟准备/重试暂存、最长60分钟ZIP、128MiB ZIP/源文件总量、2GiB任务磁盘/两个处理槽等规则保留。审计不保存Logo像素、原图、名单或正文。

### API与兼容

- 单图Go `/api/add-watermark`、`/api/workbench/dual`：新增可选multipart `output` JSON，如 `{"format":"jpeg","jpeg_quality":90}`。旧调用省略为PNG；实际响应为image/png或image/jpeg，附件名对应。`X-Watermark-Quality`给出实际最终摘要，验证/门槛失败422含受控code与quality。
- `/api/jobs/preview-plan`、`/api/jobs`：可选 `output` 及 `artwork`。Artwork包含layers（text/logo/both）、可选logo（sha256/position/width_ratio/opacity）及exclusions（源SHA→最多8个归一化矩形）。Logo仅声明身份，不上传原素材；最终渲染输入仍须逐份上传并核对摘要。
- 任务响应新增固定output/artwork；每份quality新增encoding/final/output_sha256/output_bytes/format/jpeg_quality。v2格式号不变；显式默认PNG／空文字艺术层与旧省略形式保持原指纹。

升级前备份完整数据目录及外部主密钥。**旧二进制不理解新艺术层/输出/门槛，不能用其继续新任务**；回滚须恢复升级前完整备份。CLI、legacy文件模式和旧单图批次重生成保持原行为，本轮未新增算法依赖或部署动作。逐项证据见 [验收记录](docs/feature/logo-exclusion-jpeg/verification.md)。

![Logo水印实际避让预览](image/logo-exclusion-preview.png)

![最终JPEG质量与实际摘要](image/logo-jpeg-quality.png)

## 批量溯源、策略预设与质量检查

### 批量溯源

新增 **批量溯源** 入口：一次选择最多20张PNG/JPG，每张50MiB/800万像素、总128MiB。服务端逐项真实提取并查询登记，流式返回进度；同名文件保留独立输入编号和摘要，重复内容标注首次输入，不静默丢弃。

结果区分已登记、有效但未登记、本次未检出、校验失败与处理错误。损坏图片只使该项出错；认证、配置或服务错误会明确阻止/终止本批，不能冒充“未检出”。展开可查看实际文件摘要、尺寸、载荷与水印时间，以及匹配的任务、原图和登记接收人。**登记对应接收人不等于认定谁实施了泄露**。

取消会保留已收到的结果，停止后续处理；当前不可打断的编解码/提取步骤结束后才释放服务端处理槽。没有收到结果的项目显示“未确认执行”，不能因断流断言服务端从未处理，更不能当作阴性。可显式重新发起新批次，不自动重复提交。

JSON、CSV、自包含HTML导出包含批次状态、总数、已返回数及全部输入项，取消/失败不包装为完整成功；CSV防公式执行，HTML转义且无脚本/外链。报告在浏览器临时汇总，离页/刷新前自行导出；不新增服务器长期图片/调查档案库。汇总起始时间来自浏览器，每项验证观察时间来自服务端，不是可信时间认证。审计仅记随机批次编号及服务器观察结果，不保存文件名或报告正文。

### 本机策略预设

多图任务和单图双水印共享 **水印策略**：最多20个自定义策略，加3个内置示例。支持保存、更新、重命名、删除；选择后必须点击“应用”，核对生效值并重新预览。策略只保存模式、接收人可见文字模板、样式和质量选项，不保存图片、名单、隐形正文或具体密钥。单图应用样式、Logo布局、质量和输出选项，不自动填隐形载荷；涉及Logo需重新选择文件。

保存在当前浏览器 `watermark.policies.v1`，不跨设备同步。旧 `watermark.visible.templates.v1` 样式模板继续可用。存储损坏、禁用、已满、超过上限或配置无效会明确告警，不显示保存成功；保存新策略可替换损坏列表。内置示例始终可重新选择，应用策略不修改已创建任务。

### 真实质量摘要与可选门槛

单图双水印、多图任务逐份实际提取校验后计算质量，显示尺寸、34字节载荷上限、验证结果和RGB PSNR：

- **隐形嵌入变化**：参考实际送入Go嵌入的图片（双水印时为已渲染可见层的PNG）。
- **可见层变化**：参考原始图片的服务端解码像素，单独列出，不混称隐形损伤。
- 相同像素显示∞；不同尺寸/不能比较显示“不可比较”；旧历史缺少指标显示“未记录”，不虚构旧分数。

默认仅展示质量，不新增画质拒绝条件。可显式启用最低最终成品PSNR，范围20～60dB，建议35dB。低于门槛或无法比较会拒绝成品；等于门槛或∞通过。门槛使用最终文件解码后相对嵌入输入的未舍入分数，详情显示值可能经过格式化，原值在API摘要及ZIP清单中保留。PSNR衡量像素差异，不代表人眼评价、抗压缩/裁剪或拍屏恢复保证。

失败/取消/中断任务重新上传的输入属于新的重试准备期：以最后一次上传计最多15分钟，期间定时清理不会删掉刚重传的文件；超时或服务重启会清理并换代上传令牌，需要重新上传。

任务固定质量策略快照，逐份保存结果，ZIP的manifest.csv新增quality_json列。任一副本不通过，整包失败、不开放部分下载；详情能查看失败份的实际分数。重试沿用原阈值和输入，不自动调整算法、密钥或强度来凑过门槛；调整策略应创建新任务。其他已完成登记不撤销，原图保持不变。

### 新增/扩展 API

- `POST /api/workbench/trace-batch`：multipart多值 `files`，认证/同源/限流，返回 `application/x-ndjson` 的begin/item/end帧；系统异常为fatal帧，开始流前的校验/权限失败使用非200。每批一个随机batch_id，最多20张/128MiB，整个请求129MiB硬界。复用全局两个处理槽，取得槽后才解析文件；multipart临时磁盘预算最多两个受129MiB限制的请求，与任务2GiB预算分别计量，结束清理。
- `POST /api/jobs/preview-plan`、`POST /api/jobs`：可选JSON `quality_policy: {minimum_psnr_db?: number}`；省略/空对象均表示默认展示，既有请求幂等兼容。任务响应新增 `quality_policy`、`copies[].quality`，不升级v2格式号。
- `POST /api/workbench/dual`：兼容原PNG响应。可选 `original` 原图和 `quality_policy` JSON表单；两份文件各最多50MiB，请求101MiB硬界。成功PNG/JPEG带 `X-Watermark-Quality` JSON摘要，门槛失败422返回 `code:quality_failed` 和 `quality`；旧调用不传原图则可见层指标未记录。

升级前仍需备份完整数据目录和外部主密钥。新字段可选，旧任务/旧指纹继续读取；**旧二进制不执行新质量门槛，不能用旧程序重试新质量策略任务**。回滚请恢复升级前完整备份。无新增生产依赖、外部水印库、团队权限或对外发送。

逐项验证与实际截图见 [验收记录](docs/feature/trace-policy-quality/verification.md)。

![批量溯源：逐项真实结果与汇总导出](image/trace-policy-batch.png)

![双水印：独立参考的真实质量摘要](image/trace-policy-quality.png)

## 多图分发任务与操作审计

新增 **多图分发任务** 和 **操作审计**。需启用下文的 `WATERMARK_DATA_DIR` / `WATERMARK_DATA_KEY`；未配置时原单文件功能仍可使用，但没有持久任务和审计。保留环境管理员与单实例部署；启用团队协作后可创建成员并按团队隔离。不会向接收人自动发送消息或文件。

### 创建、预览与后台执行

1. 选择最多5张PNG/JPG，填写或导入UTF-8 CSV名单（最多20人，16KiB，支持BOM）。同名不同内容允许，原始文件摘要重复会拒绝。
2. 选择仅隐形标识或双水印。双水印支持本机样式与 `{name}` / `{recipient_id}`；姓名中的占位符样文字不会递归替换。替换后每份最多80个字符且单行，文字太宽或已检出的缺字会明确提示。
3. 点击“校验并预览”。逐张图/逐个接收人生成独立可见层，可切换检查文字；未通过校验不创建任务。系统字体存在环境差异，必须检查预览；服务器使用同一渲染PNG嵌入隐形标识，不重新排版，也不作OCR文字认证。
4. 创建并开始后，每张图×每个接收人都有独立随机标识；后台生成并实际提取校验后才计入进度。成品按接收人安全目录打包，含对应清单；用户文件名/姓名不会成为ZIP路径。

显示上传、排队、生成、打包/登记、完成/失败/取消/中断。生成份数达到总数不等于包已就绪，完整ZIP及登记提交成功后才允许下载。任务在服务端执行，关页不等于取消；刷新或重新登录后仍可查询，浏览器只记住选中的任务编号，不长期保存图片/名单。会话失效会暂停后台页面轮询，迟到的旧认证响应不会使新会话退出；旧单文件草稿仍可按原规则恢复。

### 取消、恢复与文件留存

“停止本次上传”只停止浏览器本次处理；“取消后台任务”才发起服务端取消。取消在处理步骤之间生效，正在进行的单次编解码/嵌入先结束；不提供部分包下载。准备上传超过15分钟会失败并清理，需按任务快照恢复。

重启时未完成任务标为 `interrupted`，清理原图/中间输入和部分包；可载入任务参数、重新选择相同原图并生成相同预览后重试。保留原任务/副本编号、时间、密钥和模板快照，核对源文件及渲染输入摘要；字体或参数造成像素变化时明确冲突，不偷偷更换文件。取消、终态或重启恢复会换代上传令牌，拒绝旧请求迟到写回。原图/中间图在终态清理，重试容量按逻辑输入槽计数，不把重传同一文件重复累加。

成品默认自完成起暂存 **60分钟**，正常重启仍可在有效期内下载同一包。可通过 `WATERMARK_JOB_ARCHIVE_TTL_SECONDS=1..3600` 显式缩短（不能延长）留存；仅启用持久任务时校验，非法值启动失败，任务页面显示实际截止时间。到期立即禁止新下载，运行中按秒清理文件；已开始发送的响应可以完成。删除失败有维护告警，需恢复磁盘/权限后清理。缺失或损坏包显示不可用，**不撤销已完成生成的登记**。过期任务不承诺旧包重生成；需新文件可创建新任务，原单图批次的重生成能力保留。

在途切换当前密钥/修改本机模板不改变已开始任务快照；停用旧版本阻止新启动/重试，已生成文件不被销毁。副本登记查询及图片验证显示任务、源图和接收人的关联；原始文件摘要与可见层输入摘要分别标注，尚未生成的计划副本不会冒充已完成登记。

| 上限 | 行为 |
| --- | --- |
| 每任务 | 5图×20人，最多100份 |
| 原图 | 单张50MiB/800万像素、非透明、符合Go v1容量；总源文件128MiB |
| 原图＋渲染输入 | 每任务512MiB，逐份上传，不同时解码100份 |
| 完整成品 | ZIP128MiB，超限整个任务失败，不交付部分包 |
| 并发/队列 | 最多4个非终态任务，复用2个盲水印处理槽 |
| 任务文件磁盘 | 总预算2GiB；上传为后续ZIP预留空间，避免输入占满后无法执行；请求multipart/暂存另有最多两个受51MiB请求限制的生命周期 |
| 输入上传 | 独立全局240次/分钟、单份请求50MiB；创建/启动/重试/取消/下载沿用原业务动作限流 |
| 历史 | 最多1000任务、100000审计事件，并受64MiB规范快照共同限制，哪个先满就限制新写；为在途终态保留余量，不自动删除历史 |

### 审计与一致性

记录登录/退出、新旧分发生成/重试、任务取消/下载、密钥变更、提取/溯源/报告验证等业务动作。自动任务轮询、会话探测、静态资源不灌入审计。字段只有账号/匿名身份、服务端时间、动作、目标编号或文件摘要、关联编号、结果和受控错误码；不记录密码、Cookie、密钥材料、水印正文、完整名单或搜索词。匿名登录失败不保存提交的账号文本。

管理员可按日期、账号、动作和结果筛选分页，并导出当前查询范围JSON/CSV；最多10000条，超限明确拒绝而不截断，CSV防公式执行。只识别登录账号，不能区分共用账号的实际个人；“服务端已发送响应”不等于接收人实际收到，浏览器保存HTML/JSON也不伪记成服务器可确认事件。

密钥/登记/任务的关键修改和审计存于同一个v2原子快照。落盘结果不确定时限制后续关键写并显示降级告警，**不表示该操作一定未执行**；恢复存储、重启后先查询再重试。登录/安全的读取可继续或返回明确降级，审计缺口在运行状态及受控服务日志中告警，不伪造历史。元数据默认持续保留，没有在线删除审计入口或无限容量承诺；容量不足应备份/导出并规划离线迁移，不能删目录“修复”。这不是第三方可信签章或不可伪造日志。

v2备份及回滚规则见下文；`jobs-inputs/` 为可清理临时输入，`artifacts/` 为限时成品，不属于长期素材库。恢复只备份元数据时，原有生成事实仍在，未恢复的成品会明确不可用。启动验证主密钥和规范数据并取得独占锁后，清理已知中断事务临时文件；错误主密钥不清理数据。

### 任务与审计 API

均要求登录Cookie和同源写保护，公开响应没有密钥材料。

| 接口 | 输入/响应 |
| --- | --- |
| POST `/api/jobs/preview-plan` | JSON名单/模式/模板/样式，非持久校验并返回规范名单 |
| POST `/api/jobs` | JSON `sources[{name,source_sha256,width,height}]`、`recipients_csv`、`mode:blind/dual`、`template_text`、`style`；`Idempotency-Key`头，返回准备状态和固定矩阵 |
| POST `/api/jobs/:id/inputs` | multipart `file,source_id,copy_id?`；头`X-Upload-Token`来自当前任务，取消/恢复后旧令牌失效 |
| POST `/api/jobs/:id/start`、`/retry`、`/cancel` | 启动/重试/显式取消，不重复执行同一在途任务 |
| GET `/api/jobs/:id` | 状态、真实已验证进度、源/渲染快照、独立下载状态 |
| POST `/api/jobs/search` | JSON `q,page,page_size`，最多100/页 |
| GET `/api/jobs/:id/archive` | 认证ZIP响应；未就绪409、过期410、缺失/损坏明确失败 |
| GET `/api/audit/status` | 启用/降级/任务文件维护状态及配额 |
| POST `/api/audit/search` | JSON `from,to`（RFC3339）、`actor,action,outcome,page,page_size`；筛选按精确字段匹配 |
| POST `/api/audit/export` | 同筛选，`format:json/csv`；返回全部匹配（最多10000） |

验证细节见 [本轮验收记录](docs/feature/distribution-jobs/verification.md)。真实进程重启/强杀/重试/过期冒烟：`node web/frontend/scripts/restart-smoke.mjs`，全部使用独立临时目录和公开测试配置。

## 效果实验室、双水印与报告

新增 **效果实验室** 和 **双水印模板** 入口，沿用登录、主题与中英偏好，不新增水印算法库或Go依赖。既有Go DCT v1协议、分发登记及密钥轮换保持兼容。

### 效果实验室

![效果实验室](image/workbench-laboratory.png)

上传PNG/JPG原图和最多34个UTF-8字节的隐形载荷，使用当前管理密钥（未启用管理库则使用 `WATERMARK_BLIND_KEY`）。服务端从同一盲水印基线独立生成以下8项实验及2个负例，不创建真实接收人登记：

- 原样PNG；JPEG质量95、85、75。
- 最近邻缩放为75%、50%；中心保留90%宽高裁剪；顺时针旋转90度。
- 原始输入、随机错误密钥两个负例。如果原始输入已经带有有效水印，会明确标记负例异常。

每行展示实际输出尺寸、恢复结果、错误码、耗时及同尺寸PSNR；完整文本一致且验证通过才算恢复。PSNR参考原始解码RGB，不同尺寸或完全相同（无限值）显示“—”。图像预览为最多640px的JPEG缩略图，恢复判断使用完整处理输出。可下载JSON/HTML实验报告（不包含缩略图数据），报告包含源文件摘要及每项完整输出摘要。

**执行裁剪、缩放或旋转测试不等于算法支持恢复**。失败如实展示，单图结果不代表任意图片；没有模拟真实拍屏或新增几何恢复算法。原图不变，不串联多个变换。每次实验最多90秒，在处理步骤间响应取消；取消后当前不可中断编解码步骤结束才释放服务端处理槽。

### 双水印模板

![双水印预览与下载](image/workbench-dual-watermark.png)

1. 选择原图，输入可见文字和独立的隐形载荷。
2. 配置单处/平铺、位置、12～96字号、颜色、0.05～1不透明度与平铺角度，生成预览。
3. 核对可见文字，按选定格式生成并下载PNG/JPEG；服务端编码后实际提取校验，失败不交付成功下载。

可见文字支持中文/英文，最多80字符且单行。使用当前设备系统字体，有缺字或文字过宽时提示调整；系统字体会影响外观，必须检查预览。**同一份预览PNG提交给Go处理**，服务端不另行渲染字体；下载图额外包含盲水印像素变化。可见层在隐形层之前生成。

内置角落/斜向平铺样式；命名自定义模板最多20个，只在当前浏览器保存样式，不保存图片或本次文字，不跨设备同步。浏览器存储损坏/禁用/已满会提示，保存有效新模板可覆盖损坏配置。输入或样式变化使旧预览/下载失效；离开新页面取消在途请求并释放临时结果，重新进入需要重新选图。

输入沿用PNG/JPG、50MiB、800万像素、非透明与Go v1容量限制；浏览器先检查编码头尺寸再解码。不是多人多图批次模板，不改变旧批次的重生成结果。

### 验证报告

Go单图提取和分发记录的图片溯源页面都有 **验证并生成报告**，对当前所选文件独立验证。报告包含实际上传文件SHA-256、尺寸、服务端观察时间/耗时、算法、状态，以及通过验证后的载荷/水印时间、已知密钥编号和可选登记信息。

- `registered`：载荷通过验证且匹配本平台登记。
- `unregistered`：载荷通过验证，但没有对应登记。
- `not_found`：本次未检出有效水印，不能断言图中绝无水印。
- `invalid`：载荷验证失败或图片容量不足。

配置缺失、停用密钥、网络/服务异常等明确报错，不伪造“未检出”报告；单环境模式不虚构管理版本编号。JSON保留字段名，HTML自包含、无脚本/外链，可离线阅读或用浏览器打印为PDF。所有用户文字均转义。报告由用户下载保管，服务端不新增长期实验/图片/报告库。报告不是数字签章、第三方鉴定、可信时间认证或实际泄露者认定，也不证明整图内容完整。

### 新增工作流 API

均为受认证、同源保护和限流的multipart POST，复用全局最多两个处理槽。使用现有登录Cookie：

| 接口 | 字段 | 响应 |
| --- | --- | --- |
| `/api/workbench/lab` | `file`原图、`watermark` | schema_version=1的实验JSON，含缩略预览 |
| `/api/workbench/dual` | `file`已渲染可见层PNG、`watermark`隐形载荷 | 实际最终校验后的PNG/JPEG（省略output默认PNG） |
| `/api/workbench/verify` | `file`、可选`key_id` | schema_version=1的验证JSON；真实阴性/校验失败为200内状态，输入/配置/权限异常仍为非200 |

详细契约与验证见 [设计](docs/feature/watermark-workbench/design.md) 和 [验收记录](docs/feature/watermark-workbench/verification.md)。

## 批量副本、登记溯源与密钥版本

平台新增三个入口：**批量副本、分发记录、密钥版本**。按名单为同一张图片生成不同标识的副本，验证图片后查询登记信息；“登记接收人”不等于实际泄露者。平台只生成和打包文件，不自动向接收人发消息或邮件。

### 启用持久功能

```bash
# 先按下文构建前端与 ./cli，保留已有管理员配置。
export WATERMARK_DATA_DIR="$(pwd)/.watermark-data"
# 仅首次生成！请通过私密环境配置保存，后续启动必须使用同一个主密钥。
export WATERMARK_DATA_KEY="$(./cli blind-keygen)"
./server
```

`WATERMARK_DATA_KEY` 是32字节随机主密钥的64位hex表示，必须与旧 `WATERMARK_BLIND_KEY` 不同。不要在每次启动脚本里重新生成，也不要把真实值提交 Git。生产配置和备份应放在受限权限位置；服务不会自动读取 `.env`。

| 配置/状态 | 行为 |
| --- | --- |
| DATA_DIR 与 DATA_KEY 均不配置 | 旧单文件功能照常，三个新增入口提示配置未启用 |
| 只配置其中一个、主密钥错误或状态损坏 | 启动明确失败，不静默清空或重建数据 |
| 首次启用且已有有效 BLIND_KEY | 一次性导入为 `legacy-env` 当前版本，旧 v1 水印继续可验证 |
| 首次启用但无 BLIND_KEY | 安全生成初始当前版本，材料只保存于加密密钥库 |
| 已初始化后保留旧 BLIND_KEY | 必须与迁入值相同；后续轮换通过版本管理完成，不靠改旧环境变量 |

持久功能使用标准库，单实例写入，私有目录0700、文件0600；密钥字段经 AES-GCM 独立加密，与记录元数据共同原子保存于v2快照，由主密钥派生的 HMAC 保护整体完整性，两个用途派生不同密钥。使用 OS 文件锁，第二个写进程拒绝启动。本轮持久模式支持 macOS/Linux 等支持 flock 的 Unix；不支持的平台明确拒绝启用，未配置持久模式的原功能仍可运行。

### 名单与批次

一批一张符合 Go DCT v1 限制的图片，最多20名接收人。页面可直接录入或导入不超过16KB的 UTF-8 CSV（允许BOM）：

```csv
recipient_id,name
001,Alice
002,张三
003,张三
```

编号按原值区分、在本批次内不能重复；姓名允许同名。每份副本嵌入独立32位随机标识，ZIP内用标识命名PNG，另附UTF-8 `manifest.csv` 对应名单。导出对可能触发电子表格公式的字段做文本转义。原图作为不可变输入，不在上一副本上累积嵌入。

批次提交使用 `Idempotency-Key`（16–128字符，页面自动生成）：相同键/相同输入沿用原批次和标识；同键不同输入409；同批次并行请求409。整个ZIP生成并提交登记后才交付，不返回部分成功包。生成包最多128MiB，与单图共用两个处理槽；任务繁忙429。记录总量最多10000批，实际还受64MiB状态文件限制，满后明确拒绝，不自动删除旧记录。

**不长期保存原图、副本或ZIP**。下载响应结束即清理服务端临时文件，网页刷新后不提供永久下载链接。正常 SIGTERM/中断会优雅关闭并清理；异常强杀可能留下私有 runtime 目录，下一次用正确配置启动并取得独占锁后仅清理该临时目录，不删除登记或密钥文件。记录页可重新上传原始图片（SHA-256必须相同），使用原批次标识、时间与密钥重新生成ZIP，不新增分发记录。密钥停用后不能重生成，需管理员显式恢复该版本。重生成失败不撤销先前已成功生成副本的登记。

生成中断会在重启时标为失败/`interrupted`，可核对输入重试。磁盘写入失败或持久状态不确定时服务明确拒绝继续操作，需检查权限、磁盘与备份后重启，不能以删除数据目录代替修复。元数据默认持续保留，本轮没有自动删除或批量删除入口。

### 密钥生命周期与溯源

密钥版本创建后默认为**仅提取**；设为**当前嵌入**后，原当前版本自动转为仅提取。当前版本不能直接停用，先切换到其他版本。**停用**版本不参与平台验证或重生成；可显式恢复为仅提取。状态变更有确认提示，不改变已开始批次固定的密钥快照。最多8个同时可用版本、累计64个版本，网页不返回或导出任何密钥材料。

图片溯源自动尝试可用版本，也可显式选版本。Go DCT v1 的图片包没有外露密钥编号，所以自动未检出不能断言具体用了哪个已停用版本；显式选停用版本时返回 `key_disabled`。只有载荷签名、ID、密钥版本和创建时间均匹配已生成记录，才显示登记接收人。有效水印但没有对应记录会显示“未登记”，不会伪装成无水印。原单文件提取页面也展示可关联的登记信息。

旧 `legacy` 模式不变；未启用持久模式时原 Go v1 环境变量路径不变。原CLI仍使用 `WATERMARK_BLIND_KEY`，不读取管理密钥库；由管理密钥生成的副本可通过平台验证。持有原始密钥的离线程序仍可能验证旧图，平台“停用”不是从图片中销毁水印或密码学撤销。

### 新增 API

均须登录Cookie；写请求受同源保护，生成/验证/密钥修改受限流。密钥接口只有元数据。

| 接口 | 参数/响应 |
| --- | --- |
| GET `/api/platform/keys` | 版本元数据列表 |
| POST `/api/platform/keys` | 创建仅提取版本 |
| POST `/api/platform/keys/:id/state` | JSON `state`: active / decrypt-only / disabled |
| POST `/api/platform/batches` | multipart `file`、`recipients_csv`，头 `Idempotency-Key`；ZIP响应带 `X-Batch-ID` |
| POST `/api/platform/batches/:id/download` | 重新上传原图 `file`，下载同批次ZIP |
| POST `/api/platform/batches/search`、`/records/search` | JSON `q,page,page_size`，默认20，最大100；响应items/total/page/page_size |
| POST `/api/platform/trace` | multipart `file`，可选 `key_id`；已验证载荷、key_id、match及可选record |

搜索词放在请求体，常规应用日志只记录路径、状态和耗时，不记录名单或密钥。记录页有分页、错误/空状态，批次显示最近生成状态；`generated=true`表示曾完成过生成，不等于已送达接收人。

### 备份、恢复与升级

当前程序将v1数据验证后升级到v2原子快照，使业务修改与审计一同提交。升级前请停止旧服务并备份原format.json、keys.enc、records.json及外部主密钥。升级后platform.json为规范真值；旧keys.enc/records.json仅为升级时只读历史，不包含后续修改，不能拿它们替代新快照恢复。旧二进制拒绝v2标记，回滚必须恢复升级前整套备份与对应程序，不能直接运行旧程序读取新目录。

1. 停止服务，完整备份v2持久文件（format.json、platform.json）及外部 DATA_KEY；不要备份 runtime 临时图像或 .lock 文件。主密钥和持久文件分别受限保管，记录中包含接收人信息。
2. 恢复整套目录，确认目录0700、文件0600，用同一主密钥启动。不要只恢复单个文件，不手工修改签名记录或加密密钥库。
3. 主密钥错误、密钥库缺失、记录签名损坏或初始化未完成均明确拒绝。找回正确密钥/完整备份后再恢复，不清空历史。
4. 当前未提供主密钥重加密/在线轮换工具；水印密钥的版本轮换与外部 DATA_KEY 不是一回事。
5. 关闭持久配置不会删除记录，但旧程序不能读取管理库或自动获得密钥材料；回滚须保留数据、主密钥及对应程序备份。

真实进程重启冒烟（先构建前端，需要Go与Node）：

```bash
node web/frontend/scripts/restart-smoke.mjs
```

使用独立临时目录和空闲本地端口，检查重启后的记录/密钥状态、旧图溯源、幂等ZIP、错误主密钥拒绝、临时文件清理与日志不含测试接收人/主密钥。不会连接部署环境。

验证详情见 [本轮验证记录](docs/feature/distribution-traceability/verification.md)。前端测试分多套启动独立服务，保持真实默认限流不被测试覆盖；测试数据/主密钥均为临时公开测试配置，退出后清理。直接运行 Playwright 也强制隔离测试数据目录，不沿用终端中的部署目录；截图和测试输出放在系统临时目录。

## 自研 Go 图片盲水印

水印引擎全部使用 Go 标准库实现，不依赖 `blind_watermark`、Python、OpenCV 或其他水印算法库。平台的添加/提取页面可选择：

- **文件标记（兼容模式）**：保留原有文件水印操作与旧文件提取。
- **图片盲水印 · Go DCT v1**：PNG/JPG/JPEG 图片中的亮度分块 DCT 嵌入，带版本、文本长度、时间戳和 HMAC 校验；提取无需原图或外部水印长度。

![Go 自研图片盲水印提取](image/go-blind-extract.png)

### 配置密钥

```bash
go build -o cli ./cmd/cli
# 只在首次建立密钥时生成；不要每次启动都生成新密钥。
export WATERMARK_BLIND_KEY="$(./cli blind-keygen)"
```

密钥为 32 个随机字节的 hex 表示（64 位十六进制），应通过安全配置保存并备份，同一份水印必须使用原密钥提取。未启用管理库时，v1 仍为单环境密钥，换密钥后需保留旧值；可按上文迁入版本管理。密钥不进入网页或日志，不写入 Git。服务不会自动读取 `.env`。

未配置时只有新模式返回 `503 / blind_key`，兼容模式仍可用。服务器同时处理最多两个新模式任务，忙时返回 `429 / blind_busy`，可稍后重试。

### 使用与范围

```bash
# 输出路径必须不存在；拒绝覆盖原图、已有文件和指向原图的符号链接。
./cli blind-add source.jpg marked.png '项目-A100'
./cli blind-extract marked.png
# CLI 也可输出 JPEG（固定质量 95）
./cli blind-add source.jpg marked.jpg '项目-A100'
```

提取成功输出 JSON：`watermark`、UTC RFC3339 `timestamp`、`algorithm: "go-dct-v1"`。密钥错误通常表现为未检出；标记存在但版本/签名错误返回校验失败。HMAC 校验的是载荷真实性，**不加密文字，也不证明整张图片未被修改或水印无法移除**。

| 限制 | Go DCT v1 |
| --- | --- |
| 文本 | 非空有效 UTF-8，最多 34 字节，超限不截断 |
| 输入 | PNG/JPG/JPEG，最多 50 MB、800 万像素，不含透明像素 |
| 容量 | 至少 1536 个完整的 8×8 块；例如 320×320，或其他满足块数的长宽组合 |
| Web 输出 | 默认PNG，可选JPEG 60～100（默认90），最终文件重新验证；原图保持不变 |
| 元数据 | 按解码像素重新编码，不保留原图 EXIF、ICC 等元数据；需 EXIF 方向修正的图片应先规范化 |
| 几何处理 | 不承诺裁剪、缩放、旋转、截图或拍屏后恢复 |

### API

已登录的现有 multipart 接口新增可选 `algorithm`：缺省/`legacy` 保持旧行为；`go-dct-v1` 选择新模式；未知值返回 400。密钥由服务端环境或已启用的版本管理库提供。

```bash
curl -b cookies.txt -F 'algorithm=go-dct-v1' -F 'file=@source.jpg' \
  -F 'watermark=项目-A100' http://localhost:8080/api/add-watermark -o marked.png
curl -b cookies.txt -F 'algorithm=go-dct-v1' -F 'file=@marked.png' \
  'http://localhost:8080/api/extract-watermark?show_timestamp=true'
```

新模式返回 `X-Watermark-Algorithm: go-dct-v1`；提取 JSON 带 `algorithm`，时间戳按原 `show_timestamp` 参数控制。`422` 区分 `blind_not_found`、`blind_invalid`、`blind_capacity`、`blind_image`、`blind_text`；`503` 为密钥配置错误；操作仍受登录、同源和限流保护。

### 可复现的副本测试

```bash
go test -race ./...
go vet ./...
# 首次下载三张固定来源图片并核对预期 SHA-256；之后复用只读原图。
go run ./cmd/blind-bench -dir /tmp/watermark-go-bench
```

每轮建立独立 run 目录，每个压缩质量用例重新复制原图、嵌入，再从该水印输出派生压缩副本，不累积损伤。处理前后核对原图 SHA-256。报告记录样本来源、哈希、尺寸、完整载荷恢复、负例、PSNR 和含文件操作的耗时；基线或负例失败会返回非零退出码。测试使用固定公开测试密钥和固定载荷时间，不能用于生产。

当前三个样本（花朵、人像、咖啡静物）在 PNG 重新编码与 JPEG 95/85/75 下均恢复了载荷，无水印/错误密钥负例均拒绝；这只是样本结果，不是对任意图片的保证。画质改变和失败边界需结合自己的图片验证。见 [实测报告](docs/feature/go-blind-watermark/benchmark.md) 与 [验证记录](docs/feature/go-blind-watermark/verification.md)。下载图片和测试产物不入库；自制合成测试图与 v1 兼容夹具入库供离线回归。

## 平台界面与登录

前端使用 React + TypeScript + Ant Design，提供工作台、添加水印、提取水印和设置页面，支持响应式侧边导航、浅色/深色/跟随系统主题及简体中文/English。导航采用两级分组，工作台为独立入口：水印制作（添加水印、双水印模板）、分发管理（多图分发任务、批量副本、分发记录）、验证分析（提取水印、批量溯源、效果实验室）、系统管理（密钥版本、操作审计、设置）。分组箭头收起向右、展开向下。当前页面自动展开所属分组；桌面折叠使用浮层，手机抽屉保持完整菜单。导航独立滚动，底部状态与折叠按钮固定。原有 hash 地址兼容。显示偏好保存在当前浏览器；不保存密码或登录令牌到 localStorage。启用持久功能后保存分发登记元数据，原图与生成文件不长期存储。

### 表单与上传工作区

“双水印模板”和“多图分发任务”按 **上传素材 → 配置水印 → 预览确认 → 生成与下载** 分区。基础输入直接展示；样式、避让区域、策略管理、输出格式与质量门槛位于默认收起的“高级配置”，展开/收起保留输入。

宽度大于1100px时配置与预览并排，右侧预览随页面保持可见，内容较长时可在预览区内滚动。窄窗口、平板和手机使用“配置 / 预览与生成”切换，可用底部入口与“返回配置”；切换保留素材、字段和结果。修改素材或配置后须重新预览，可见层预览不代表最终隐形水印已验证。

原图、Logo、溯源图片、CSV与重建ZIP的素材入口统一支持选择/拖拽、文件名/格式/大小、图片缩略图、替换和删除。多文件选择替换本次文件组，拖入或“添加文件”可追加，支持逐项替换/删除。非法选择会显示原因并保留原有效选择；操作中禁止更换素材。限额沿用各功能原有规则：图片每份50MiB，Logo 5MiB、CSV 16KiB，多图任务5张、批量溯源20张，两者图片合计128MiB；实际内容仍由既有处理链验证。

选择文件不会自动上传或触发水印生成；CSV读取与缩略图在本机完成，原文件不覆盖。CSV导入后的名单文字可以继续编辑，删除CSV文件引用不清空文字。缩略图仅在可见时占用临时URL，隐藏或替换/移除时释放；保留草稿的页面再次显示时恢复缩略图。

![双水印桌面配置与预览并排](docs/feature/form-workspace/screenshots/dual-split-desktop.png)

![登录界面](image/platform-login.png)

### 构建与启动（平台版）

需要 Go 1.24.4+、Node.js 20.19+ 或 22.12+、npm。首次启动前必须构建前端：

```bash
cd web/frontend
npm ci
npm run build
cd ../..
go build -o server ./cmd/server
export WATERMARK_ADMIN_USER=admin
# 请设置自己的密码，至少 12 字节；下面交互输入不会写入命令历史。
read -s WATERMARK_ADMIN_PASSWORD
export WATERMARK_ADMIN_PASSWORD
./server
```

从仓库根目录启动并访问 http://localhost:8080。服务提供本地 `web/dist` 资源，不依赖浏览器 CDN。每次修改前端后重新构建。`web/dist` 不提交到 Git，部署时需与二进制一起复制；无需运行 Node 服务。

| 环境变量 | 说明 |
| --- | --- |
| `WATERMARK_ADMIN_USER` | 必填，环境管理员（引导/恢复账号）用户名 |
| `WATERMARK_ADMIN_PASSWORD` | 必填，至少 12 字节；不提供默认密码 |
| `WATERMARK_COOKIE_SECURE` | HTTPS 部署设 `true`；本地 HTTP 默认 `false` |
| `PORT` | 默认 `8080` |

配置缺失时登录返回 503，页面显示配置说明。`.env.example` 仅作变量参考，服务不会自动读取 `.env`。生产环境应通过 HTTPS 提供平台，并配置安全 Cookie。服务默认不信任代理提供的客户端 IP；代理部署需按实际网络配置可信代理后再按客户端 IP 限流。

会话使用随机令牌、HttpOnly / SameSite=Strict Cookie，最长 8 小时，退出即撤销，服务重启后失效。当前登录为单实例内存会话，不支持多副本共享登录；分发记录与密钥版本按显式持久配置保存。环境管理员密码变更需重启服务。启用持久存储后，管理员可在“团队与成员”创建账号、分配管理者/制作操作员/审阅员、停用及重置成员密码；成员会话在禁用或密码版本变化后立即失效。没有开放注册和自助密码找回。

### 团队空间与成员

需配置现有 `WATERMARK_DATA_DIR` / `WATERMARK_DATA_KEY` 及环境管理员。原数据保留在仅管理员可访问的“个人工作区”；团队使用独立目录、密钥与任务引擎，切换空间会清空未提交素材和页面状态。浏览器样式模板、本机策略和当前任务选择按账号/团队分开，个人工作区兼容旧key。

环境管理员创建账号和团队，团队管理者可管理本团队成员及密钥；制作操作员可制作/分发，审阅员只读与验证。服务端逐请求校验当前角色，隐藏菜单不代替权限。账号密码为12～72字节，bcrypt保存摘要；名称为1～64位ASCII字母/数字开头及字母数字、`_.@-`，大小写区分。账号/团队修改必须提供revision；PUT启用状态与成员role必须显式给出，移除成员用role空字符串。

协作元数据存于 `collaboration/workspace-v1.enc`，AES-GCM加密、0700目录/0600文件、进程锁和原子提交。上限256成员账号、32空间（含个人），16MiB状态/10万事件，预留1MiB与512事件供环境管理员撤权。既有团队引擎记录沿用v2认证元数据与加密密钥格式，所在目录私有，不宣称旧记录格式全量加密。新团队不复用个人空间密钥；全局处理槽仍有界。

API位于 `/api/workspace/me`、`/users`、`/teams`、`/teams/:id/members/:username`（均在workspace前缀内），团队处理路径为 `/api/workspace/teams/:id/engine/api/...`，复用已有任务、分发、提取、审计与密钥接口。根处理接口仍限环境管理员，避免新成员看到旧全局记录。未配置持久存储时保留原单文件功能，团队页面显示配置提示。

升级前备份整个数据目录和外部主密钥。环境管理员名称与协作数据绑定，不得仅改用户名变量来重新归属旧团队；保持原名可更换环境密码。旧二进制不提供新协作能力；回滚应停服务并恢复升级前完整备份，不混用新旧团队文件。本轮新增能力按 [完整进度板](docs/feature/platform-expansion/implementation-plan.md) 分批实施；团队地基不代表其余清单已经完成。

### 开发与验证

```bash
# 终端 1：配置上述账号变量后，在仓库根目录运行
 go run ./cmd/server
# 终端 2：开发服务器（代理 /api 至本地 :8080）
 cd web/frontend
 npm run dev

# 仓库根目录运行 Go 检查
 go test -race ./...
# 前端目录运行
 npm run build
 npx playwright install chromium
 npm run test:e2e
```

E2E 使用专用本地 :18080 服务与测试账号，执行前需已构建前端，并保证端口空闲。测试覆盖登录、访问控制、文件往返、偏好保存和移动端导航。开发代理以 Vite 默认转发原始 Host 的方式保持同源检查。

### API 登录迁移（兼容性变化）

水印 API 路径、参数及成功响应不变，但现在必须先登录并携带会话 Cookie，匿名请求返回 401。CLI 使用不变。浏览器跨站写请求返回 403。登录与文件操作共享每 IP 每分钟 10 次限制，静态资源、会话探测、格式查询和退出不计入额度。

```bash
# 将凭据 JSON 写入本地受限权限文件，不将真实密码放进版本库。
curl -c cookies.txt -H 'Content-Type: application/json' \
  --data-binary @credentials.json http://localhost:8080/api/auth/login
curl -b cookies.txt http://localhost:8080/api/auth/session
curl -b cookies.txt -F 'file=@文档.pdf' -F 'watermark=内部资料' \
  http://localhost:8080/api/add-watermark -o watermarked.pdf
curl -b cookies.txt -F 'file=@watermarked.pdf' \
  'http://localhost:8080/api/extract-watermark?show_timestamp=true'
curl -b cookies.txt -X POST http://localhost:8080/api/auth/logout
```

`credentials.json` 格式为 `{"username":"你的账号","password":"你的密码"}`。Cookie jar 包含会话凭据，退出后删除本地临时文件。新增接口：`POST /api/auth/login`、`GET /api/auth/session`、`POST /api/auth/logout`。当前单文件最大 50 MB，水印文本最大 **100 个 UTF-8 字节**（并非 100 个中文字符）。

## 功能界面

以下为当前 Ant Design 平台的实际操作截图。

### 添加隐水印

![添加隐水印：上传文件、输入标识并下载结果](image/platform-add-watermark.png)

### 提取隐水印

![提取隐水印：查看水印内容与添加时间](image/platform-extract-watermark.png)

### 批量副本与登记溯源

![批量副本：导入名单并下载独立副本ZIP](image/distribution-batch.png)

![图片溯源：验证后显示副本登记信息](image/distribution-trace.png)

![移动端密钥版本管理（英文）](image/distribution-keys-mobile.png)

### 多图后台任务与操作审计

![多图任务：两图三人六份验证完成，可下载整套ZIP](image/distribution-jobs-complete.png)

![操作审计：按动作查询及导出](image/distribution-jobs-audit.png)

## 主要特性

- **两种处理模式**：文件隐藏标记与图片内容盲水印，按场景选择，效果以实测为准
- **载荷校验**：Go DCT v1 使用配置密钥验证水印载荷，兼容模式能力因格式而异
- **明确的提取结果**：新模式区分未检出、版本或签名无效、容量不足等情况
- **多格式支持**：兼容多种常用文件格式（PDF、DOCX、XLSX、PPTX、JPG、PNG等）
- **简洁界面**：直观易用的Web界面，操作简单快捷
- **API支持**：提供完整REST API，便于集成到现有系统
- **资源边界**：新模式限制文件体积、解码像素与并行任务数，超限明确拒绝

## 支持的文件格式

| 文件类型 | 水印添加 | 水印提取  | 备注 |
|---------|:-------:|:-------:|------|
| PDF     | ✅      | ✅      | 元数据隐写技术，文档外观无变化 |
| DOCX    | ✅      | ✅      | 在文档属性和内容中添加隐藏标记 |
| XLSX    | ✅      | ✅      | 在电子表格内部 XML 中添加标记 |
| PPTX    | ✅      | ✅      | 在幻灯片XML中添加不可见注释 |
| JPG     | ✅      | ✅      | 兼容模式使用 JPEG 注释；新模式写入图像内容 |
| PNG     | ✅      | ✅      | 兼容模式使用文件尾标记；新模式写入图像内容 |
| RTF     | ✅      | ✅      | 使用特殊字段隐藏水印信息 |
| ODT     | ✅      | ✅      | 在文档XML结构中添加隐藏标记 |

## 使用指南

### Web界面使用

1. 访问 http://localhost:8080，使用配置的管理员账号登录
2. 从侧边栏选择“水印制作 → 添加水印”或“验证分析 → 提取水印”
3. 上传文件并输入隐水印文本（添加水印时）
4. 等待处理完成后下载文件或查看提取结果

### 命令行(CLI)使用

除了Web界面外，本工具还提供命令行界面(CLI)，方便在脚本中使用或批处理文件。

#### 构建CLI工具

```bash
# 构建CLI工具
go build -o cli ./cmd/cli
```

构建完成后，CLI工具位于 `./cli`。

#### CLI命令

1. 添加水印

```bash
./cli add [输入文件] [输出文件] [水印文本]

# 示例
./cli add 文档.pdf 带水印.pdf "机密文件-请勿外传"
```

2. 提取水印

```bash
./cli extract [输入文件]

# 示例
./cli extract 带水印.pdf
```

3. 查看支持的文件类型

```bash
./cli types
```

### API接口使用

以下请求均需按前述迁移说明携带 `-b cookies.txt`，编程示例同样需要 Cookie jar 并预先登录。

#### 添加隐水印

```http
POST /api/add-watermark
Content-Type: multipart/form-data

参数:
- file: 文件数据
- watermark: 隐水印文本
```

示例请求：

```bash
curl -b cookies.txt -X POST -F "file=@文档.pdf" -F "watermark=机密文件" \
     http://localhost:8080/api/add-watermark -o 带水印文档.pdf
```

#### 提取隐水印

```http
POST /api/extract-watermark
Content-Type: multipart/form-data

参数:
- file: 文件数据
```

示例请求：

```bash
curl -b cookies.txt -X POST -F "file=@带水印文档.pdf" \
     http://localhost:8080/api/extract-watermark
```

#### 获取支持的文件类型

```http
GET /api/supported-types
```

示例响应：

```json
{
  "types": ["pdf", "docx", "xlsx", "pptx", "jpg", "png", "rtf", "odt"]
}
```

### 编程示例

以下是通过Go代码使用该工具的简单示例：

```go
package main

import (
	"bytes"
	"fmt"
	"encoding/json"
	"net/http/cookiejar"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// 添加隐水印示例
	filePath := "example.pdf"
	watermarkText := "机密文档-请勿外传"
	
	err := addWatermark(filePath, watermarkText)
	if err != nil {
		fmt.Printf("添加隐水印失败: %v\n", err)
		return
	}
	
	// 提取隐水印示例
	watermarkedFile := "example_watermarked.pdf"
	text, err := extractWatermark(watermarkedFile)
	if err != nil {
		fmt.Printf("提取隐水印失败: %v\n", err)
		return
	}
	
	fmt.Printf("提取的隐水印内容: %s\n", text)
}

// 添加隐水印
func addWatermark(filePath, watermarkText string) error {
	// 创建multipart表单
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// 添加文件
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}
	
	// 添加水印文本
	err = writer.WriteField("watermark", watermarkText)
	if err != nil {
		return err
	}
	
	writer.Close()
	
	// 发送请求
	resp, err := httpClient.Post("http://localhost:8080/api/add-watermark", writer.FormDataContentType(), &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回错误: %s", resp.Status)
	}
	
	// 保存结果
	output, err := os.Create(filePath[:len(filePath)-4] + "_watermarked" + filepath.Ext(filePath))
	if err != nil {
		return err
	}
	defer output.Close()
	
	_, err = io.Copy(output, resp.Body)
	return err
}

// 提取隐水印
func extractWatermark(filePath string) (string, error) {
	// 创建multipart表单
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// 添加文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}
	
	writer.Close()
	
	// 发送请求
	resp, err := httpClient.Post("http://localhost:8080/api/extract-watermark", writer.FormDataContentType(), &buf)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("服务器返回错误: %s", resp.Status)
	}
	
	// 读取结果
	var result struct {
		Watermark string `json:"watermark"`
		Error     string `json:"error"`
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	return string(body), nil
}
```

## 水印技术原理

兼容模式在文件属性、XML 或注释/文件尾等位置保存标记，不同格式的编码和保护方式不同，经过重新保存或元数据清理后可能丢失。

Go DCT v1 在 8×8 亮度块中投影两个正交 DCT 系数，通过系数差的正负嵌入比特。密钥派生的块顺序分散信息，重复编码并合并限幅证据恢复载荷；最后验证版本、长度和 HMAC。重复恢复用于抗噪声，HMAC 用于载荷校验，两者用途不同。具体协议见 [设计记录](docs/feature/go-blind-watermark/design.md)。

## 安全与能力边界

- Web 操作需要登录，生产环境应使用 HTTPS 与安全 Cookie。
- Go DCT v1 使用服务端配置密钥对载荷做 HMAC 校验；不提供载荷加密、整图完整性认证或不可移除保证。
- 兼容模式因格式而异：PNG/PDF 部分实现为 Base64 标记，JPG 使用旧版固定密钥。不能将所有兼容格式统一描述为强加密或防篡改。
- 旧格式时间信息按各处理器实现返回，不作为可信时间证明；自研模式的时间戳受载荷 HMAC 保护，但不等于第三方时间认证。
- 水印仅作为辅助标识手段，应结合访问控制、分发记录与具体业务流程使用。

## 注意事项

1. 图片内容盲水印会改变像素，使用前应评估画质与恢复率；不承诺不可检测
2. 水印安全性依赖于密钥的保密性，请妥善保管系统密钥
3. 不同文件格式的水印实现方式不同，安全级别可能有差异
4. 处理大型文件时可能需要调整服务器配置，增加超时时间

## 未来计划

- [ ] 支持更多文件格式（包括音频和视频格式）
- [x] 图片按接收人生成批量副本；其他格式批处理待扩展
- [ ] 实现水印模板和权限管理
- [ ] 提供更高级的水印策略（如动态水印、分层水印）
- [ ] 开发桌面客户端版本

## 许可证

本项目采用 MIT 许可证 - 详见 LICENSE 文件 
