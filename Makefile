.PHONY: build clean run-server run-cli help test

# 目标文件夹
BUILD_DIR = ./build
# 命令行工具输出
CLI_OUTPUT = $(BUILD_DIR)/watermark-cli
# Web服务器输出
SERVER_OUTPUT = $(BUILD_DIR)/watermark-server

# 默认目标
all: build

# 构建全部
build: $(CLI_OUTPUT) $(SERVER_OUTPUT)

# 构建命令行工具
$(CLI_OUTPUT):
	@echo "构建命令行工具..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(CLI_OUTPUT) ./cmd/cli

# 构建Web服务器
$(SERVER_OUTPUT):
	@echo "构建Web服务器..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(SERVER_OUTPUT) ./cmd/server

# 清理构建产物
clean:
	@echo "清理构建产物..."
	@rm -rf $(BUILD_DIR)

# 运行Web服务器
run-server: $(SERVER_OUTPUT)
	@echo "启动Web服务器..."
	@$(SERVER_OUTPUT)

# 运行命令行工具
run-cli: $(CLI_OUTPUT)
	@echo "运行命令行工具..."
	@$(CLI_OUTPUT)

# 命令行工具：添加水印
add-watermark: $(CLI_OUTPUT)
	@$(CLI_OUTPUT) add $(filter-out $@,$(MAKECMDGOALS))

# 命令行工具：提取水印
extract-watermark: $(CLI_OUTPUT)
	@$(CLI_OUTPUT) extract $(filter-out $@,$(MAKECMDGOALS))

# 命令行工具：显示支持的文件类型
types: $(CLI_OUTPUT)
	@$(CLI_OUTPUT) types

# 运行测试
test:
	@echo "运行测试..."
	@go test -v ./...

# 帮助信息
help:
	@echo "水印工具 Makefile 帮助"
	@echo ""
	@echo "可用命令:"
	@echo "  make build           - 构建命令行工具和Web服务器"
	@echo "  make clean           - 清理构建产物"
	@echo "  make run-server      - 运行Web服务器"
	@echo "  make run-cli         - 运行命令行工具"
	@echo "  make add-watermark   - 添加水印 (例如: make add-watermark input.pdf output.pdf '水印文本')"
	@echo "  make extract-watermark - 提取水印 (例如: make extract-watermark input.pdf)"
	@echo "  make types           - 显示支持的文件类型"
	@echo "  make test            - 运行测试"
	@echo "  make help            - 显示此帮助信息"

# 允许将未知目标作为参数传递给命令
%:
	@: 