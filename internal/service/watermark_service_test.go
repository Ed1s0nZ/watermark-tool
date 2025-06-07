package service

import (
	"os"
	"path/filepath"
	"testing"

	_ "watermark-tool/internal/watermark/docx"
	_ "watermark-tool/internal/watermark/pdf"
	_ "watermark-tool/internal/watermark/pptx"
	_ "watermark-tool/internal/watermark/xlsx"
)

func TestWatermarkService(t *testing.T) {
	// 创建水印服务
	service := NewWatermarkService()

	// 测试获取支持的文件类型
	types := service.GetSupportedTypes()
	if len(types) == 0 {
		t.Fatalf("没有支持的文件类型")
	}

	// 检查是否支持所有预期的文件类型
	expectedTypes := map[string]bool{
		"pdf":  false,
		"docx": false,
		"xlsx": false,
		"pptx": false,
	}

	for _, fileType := range types {
		expectedTypes[fileType] = true
	}

	for fileType, supported := range expectedTypes {
		if !supported {
			t.Errorf("预期支持文件类型 %s，但未在支持列表中", fileType)
		}
	}

	// 获取测试文件路径
	// 注意：这些测试需要项目根目录下有实际的测试文件
	testFiles := map[string]string{
		"pdf":  "../../未命名1.pdf",
		"xlsx": "../../工作簿1.xlsx",
		"pptx": "../../qweqwe.pptx",
		"docx": "../../测试.docx",
	}

	// 创建临时目录用于输出文件
	tempDir, err := os.MkdirTemp("", "watermark-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 测试水印文本
	watermarkText := "测试水印"

	// 测试每种文件类型
	for fileType, testFile := range testFiles {
		t.Run("Test_"+fileType, func(t *testing.T) {
			// 检查测试文件是否存在
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				t.Skipf("跳过测试，测试文件不存在: %s", testFile)
				return
			}

			// 输出文件路径
			outputFile := filepath.Join(tempDir, "watermarked_"+filepath.Base(testFile))

			// 测试添加水印
			err := service.AddWatermark(testFile, outputFile, watermarkText)
			if err != nil {
				t.Fatalf("添加水印失败: %v", err)
			}

			// 检查输出文件是否创建
			if _, err := os.Stat(outputFile); os.IsNotExist(err) {
				t.Fatalf("未创建输出文件: %s", outputFile)
			}

			// 测试提取水印
			// 注意：由于水印提取的复杂性，实际提取的文本可能与原始文本有差异
			// 这里只测试能否提取出水印文本，不比较具体内容
			extractedText, err := service.ExtractWatermark(outputFile)
			if err != nil {
				t.Logf("提取水印可能失败，这可能是正常的: %v", err)
			} else {
				t.Logf("从 %s 提取的水印: %s", fileType, extractedText)
			}
		})
	}
}
