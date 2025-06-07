package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"watermark-tool/internal/service"
	_ "watermark-tool/internal/watermark/docx"
	_ "watermark-tool/internal/watermark/jpg"
	_ "watermark-tool/internal/watermark/odt"
	_ "watermark-tool/internal/watermark/pdf"
	_ "watermark-tool/internal/watermark/png"
	_ "watermark-tool/internal/watermark/pptx"
	_ "watermark-tool/internal/watermark/rtf"
	_ "watermark-tool/internal/watermark/xlsx"
)

func main() {
	// 创建服务
	watermarkService := service.NewWatermarkService()

	// 创建根命令
	rootCmd := &cobra.Command{
		Use:   "watermark-tool",
		Short: "办公文档水印工具",
		Long:  "一个用于添加和提取Office文档水印的工具，支持PDF、Word、Excel和PowerPoint格式。",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// 添加水印命令
	addCmd := &cobra.Command{
		Use:   "add [input_file] [output_file] [watermark_text]",
		Short: "为文档添加水印",
		Long:  "为指定的文档添加水印，支持的格式包括: " + fmt.Sprintf("%v", watermarkService.GetSupportedTypes()),
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			inputFile := args[0]
			outputFile := args[1]
			watermarkText := args[2]

			fmt.Printf("正在为文件 %s 添加水印...\n", inputFile)
			err := watermarkService.AddWatermark(inputFile, outputFile, watermarkText)
			if err != nil {
				fmt.Printf("添加水印失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("成功添加水印并保存到 %s\n", outputFile)
		},
	}

	// 提取水印命令
	extractCmd := &cobra.Command{
		Use:   "extract [input_file]",
		Short: "从文档中提取水印",
		Long:  "从指定的文档中提取水印，支持的格式包括: " + fmt.Sprintf("%v", watermarkService.GetSupportedTypes()),
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			inputFile := args[0]
			showTimestamp, _ := cmd.Flags().GetBool("timestamp")

			fmt.Printf("正在从文件 %s 中提取水印...\n", inputFile)

			if showTimestamp {
				watermarkText, timestamp, err := watermarkService.ExtractWatermarkWithTimestamp(inputFile)
				if err != nil {
					fmt.Printf("提取水印失败: %v\n", err)
					os.Exit(1)
				}

				// 转换时间戳格式
				var formattedTimestamp string
				if unixTimestamp, err := parseTimestamp(timestamp); err == nil {
					formattedTimestamp = unixTimestamp.Format("2006-01-02 15:04:05")
				} else {
					formattedTimestamp = timestamp
				}

				fmt.Printf("提取的水印文本: %s\n", watermarkText)
				fmt.Printf("水印添加时间: %s\n", formattedTimestamp)
			} else {
				watermarkText, err := watermarkService.ExtractWatermark(inputFile)
				if err != nil {
					fmt.Printf("提取水印失败: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("提取的水印文本: %s\n", watermarkText)
			}
		},
	}

	// 添加时间戳选项
	extractCmd.Flags().BoolP("timestamp", "t", false, "显示水印添加时间")

	// 列出支持的文件类型命令
	listTypesCmd := &cobra.Command{
		Use:   "types",
		Short: "列出支持的文件类型",
		Long:  "列出工具支持的所有文件类型",
		Run: func(cmd *cobra.Command, args []string) {
			types := watermarkService.GetSupportedTypes()
			fmt.Println("支持的文件类型:")
			for _, t := range types {
				fmt.Printf("- .%s\n", t)
			}
		},
	}

	// 将命令添加到根命令
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(listTypesCmd)

	// 执行命令
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// parseTimestamp 尝试将时间戳字符串解析为时间
func parseTimestamp(timestamp string) (time.Time, error) {
	// 尝试解析为 Unix 时间戳（秒）
	if unixTime, err := parseSingleValue(timestamp); err == nil {
		return time.Unix(unixTime, 0), nil
	}

	// 尝试解析为 RFC3339 格式
	if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
		return t, nil
	}

	// 无法解析
	return time.Time{}, fmt.Errorf("无法解析时间戳: %s", timestamp)
}

// parseSingleValue 尝试将字符串解析为整数
func parseSingleValue(value string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(value, "%d", &result)
	return result, err
}
