# 表单工作区设计

## 组件与数据流

FileUpload接收受控File[]、accept、maxSize/maxCount/maxTotal、disabled及onChange。以原生隐藏input作为选择器、Ant Design按钮/标签/警告呈现统一拖放区和文件行；原生input的aria-label保持现有测试/无障碍名称。共享校验整组原子拒绝，保留旧输入；扩展名/MIME按accept判断，实际解码仍由既有处理链验证。多文件选择默认替换整组，显式“添加文件”和拖入支持追加，逐项替换/删除；重复字节不同文件名不擅自去重。缩略图仅图片创建objectURL，失败回退文件图标，每项useEffect清理。

FormWorkspace使用相同配置/预览节点，桌面CSS Grid双列，右侧sticky top0并限制视口高度、内部滚动；宽度≤1100px的窄窗口/平板/手机用带标签的Segmented控制两面板hidden，不卸载节点，底部“预览与生成”与返回按钮可切换。左右数字阶段标题，输入/配置与预览/最终交付区分。mobile切换时滚到工作区顶部，尊重reduced-motion。

AdvancedSettings基于AntD Collapse，默认收起、forceRender保留原有策略加载/字段状态。双水印和多图任务把基础上传与水印文字/载荷/模式留在配置区；高级包含原有PolicyControls、VisibleStyleFields、ExclusionEditor、OutputFields、QualityFields及样式模板管理。所有配置仍由原页面持有，沿用reset/change取消与旧结果失效。预览、生成、取消按钮和错误置于右侧，生成前仍要求当前预览；结果和质量随右侧呈现。多图任务详情与历史保留原位置，不做额外列表重构。

其他页面只替换上传入口，不改变API。CSV导入保留16KiB与UTF-8校验，删除文件引用不清空文本；原图/Logo移除调用既有reset/change；重建ZIP素材变更清旧归档与错误，防止旧包冒充新素材。

## 兼容、状态与失败

不改Go/API/hash/数据格式/持久化。客户端限制与现有接口一致；拒绝不上传、不置换旧素材。忙时上传区、替换/删除/追加和配置字段禁用；切换视图和查看状态仍可用。无预览显示中性占位，不暗示已验证；操作失败保留输入，允许回配置修正。预览变更仍须用户重新触发，不宣称实时或最终隐形验证。

双语文案集中在workspaceText.ts，根据现有I18n语言派生；不新增主题系统与依赖。
