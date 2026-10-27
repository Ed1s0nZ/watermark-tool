import { useText, zh } from "./i18n";
const cn = {
  layers: "可见层组合",
  text: "仅文字",
  logo: "仅Logo",
  both: "文字＋Logo",
  logoFile: "Logo文件",
  logoHint:
    "PNG/JPG最多5MiB/200万像素，可透明；Logo只临时使用，应用策略后需重新选择。",
  logoPosition: "Logo位置",
  ratio: "Logo宽度比例",
  opacity: "Logo不透明度",
  topLeft: "左上",
  topRight: "右上",
  center: "居中",
  bottomLeft: "左下",
  bottomRight: "右下",
  output: "成品格式",
  jpegQuality: "JPEG编码质量",
  outputHint:
    "默认PNG；JPEG按所选质量编码后重新提取和检查最终质量，失败不自动换格式或提高质量。",
  regions: "可见水印避让区域",
  source: "避让原图",
  regionHint:
    "逐图设置，最多8个矩形。拖动鼠标/手指框选，或编辑百分比；只约束可见层，隐形嵌入和JPEG仍可能改变区域像素。",
  clear: "清空本图区域",
  remove: "删除区域",
  number: "区域",
  empty: "先选择原图，再框选避让区域。",
  invalid: "Logo、布局或区域无效，请检查尺寸、透明度和矩形范围。",
  blocked: "可见水印没有可用完整位置，请缩小水印或调整避让区域。",
  logoMismatch: "恢复任务需要相同Logo，请核对摘要并重新选择。",
  expected: "恢复Logo摘要",
  embedding: "隐形嵌入变化",
  encoding: "编码变化（参考嵌入后像素）",
  final: "最终成品变化（参考嵌入输入，门槛基于此）",
  bytes: "成品字节数",
  digest: "成品SHA-256",
  notVerified: "最终文件提取验证未通过",
  snapshot: "固定可见层与导出快照",
  newPolicyHint:
    "策略只保存Logo布局和输出选项，不保存Logo文件、摘要或逐图矩形。",
};
const en: typeof cn = {
  layers: "Visible layer combination",
  text: "Text only",
  logo: "Logo only",
  both: "Text and logo",
  logoFile: "Logo file",
  logoHint:
    "PNG/JPG up to 5MiB/2MP; transparency allowed. Logo is temporary; select it again after applying a policy.",
  logoPosition: "Logo position",
  ratio: "Logo width ratio",
  opacity: "Logo opacity",
  topLeft: "Top left",
  topRight: "Top right",
  center: "Center",
  bottomLeft: "Bottom left",
  bottomRight: "Bottom right",
  output: "Output format",
  jpegQuality: "JPEG encoding quality",
  outputHint:
    "PNG by default. JPEG is decoded and verified at the selected quality; failures never silently change format or quality.",
  regions: "Visible watermark exclusion regions",
  source: "Exclusion source image",
  regionHint:
    "Up to 8 rectangles per image. Drag with mouse/touch or edit percentages. Only visible layers are excluded; hidden embedding and JPEG may still change pixels.",
  clear: "Clear image regions",
  remove: "Remove region",
  number: "Region",
  empty: "Select source images, then draw exclusion rectangles.",
  invalid:
    "Invalid logo, layout or region. Check size, opacity and rectangle bounds.",
  blocked:
    "No complete visible watermark instance fits. Reduce size or adjust exclusion regions.",
  logoMismatch:
    "Retry requires the same logo. Check the digest and select again.",
  expected: "Expected logo digest",
  embedding: "Embedding change",
  encoding: "Encoding change (reference: marked pixels)",
  final: "Final file change (reference: embedding input; gate uses this)",
  bytes: "Output bytes",
  digest: "Output SHA-256",
  notVerified: "Final file extraction did not verify",
  snapshot: "Fixed artwork and output snapshot",
  newPolicyHint:
    "Policies store layout and output options, never logo files, digests or per-image rectangles.",
};
export function useArtText() {
  return useText() === zh ? cn : en;
}
