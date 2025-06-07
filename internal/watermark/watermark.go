package watermark

// Watermarker 定义了水印接口
type Watermarker interface {
	// AddWatermark 添加水印到文档
	AddWatermark(inputFile, outputFile, watermarkText string) error

	// ExtractWatermark 从文档中提取水印
	// 返回值: 水印文本, 时间戳, 错误
	ExtractWatermark(inputFile string) (string, string, error)

	// GetSupportedType 获取支持的文件类型
	GetSupportedType() string
}

// WatermarkRegistry 包含所有已注册的水印处理器
var WatermarkRegistry = make(map[string]Watermarker)

// RegisterWatermarker 注册水印处理器
func RegisterWatermarker(w Watermarker) {
	fileType := w.GetSupportedType()
	WatermarkRegistry[fileType] = w
}

// GetWatermarker 根据文件类型获取水印处理器
func GetWatermarker(fileType string) (Watermarker, bool) {
	w, ok := WatermarkRegistry[fileType]
	return w, ok
}
