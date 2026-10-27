# 实施与验证计划

1. 新建workspaceText、FileUpload、FormWorkspace/阶段/高级配置组件及响应式样式。
2. DualPage/JobsPage按四阶段重排，保留业务状态与原生成链；共享工作区提供桌面粘性预览和手机切换。
3. WatermarkPage/BatchPage/TraceForm/TraceBatchPage/LabPage/RecordsPage/ArtworkControls及CSV入口接统一上传，保留现有限额、回调和错误处理。
4. 更新已有真实浏览器用例，使高级字段与手机面板通过实际UI展开；新增专项覆盖上传校验/追加/替换/删除/拖拽/禁用、预览失效、移动保值及粘性布局。
5. TypeScript/Vite构建，专项后全套E2E；截图打开检查中文桌面、英文暗色手机和上传错误；核对每条REQ/AC与所有上传入口。
6. README、日期分类CHANGELOG、verification与阶段记录分阶段提交推送main。无后端/数据变化，回滚前端提交即可；不部署、不新增版本。
