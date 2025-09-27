# LXC Images Builder Makefile

.PHONY: help build clean test install deps lint format

# 默认目标
help:
	@echo "LXC Images Builder - 可用命令:"
	@echo "  build     - 构建Go程序"
	@echo "  clean     - 清理构建文件"
	@echo "  test      - 运行测试"
	@echo "  install   - 安装依赖"
	@echo "  deps      - 下载Go依赖"
	@echo "  lint      - 代码检查"
	@echo "  format    - 代码格式化"
	@echo "  run       - 运行构建程序"
	@echo "  docker    - 使用Docker构建"

# 构建程序
build:
	@echo "构建LXC镜像构建器..."
	go build -o lxc-builder main.go
	@echo "构建完成: lxc-builder"

# 清理
clean:
	@echo "清理构建文件..."
	rm -f lxc-builder
	rm -rf output/
	@echo "清理完成"

# 运行测试
test:
	@echo "运行测试..."
	go test -v ./...

# 安装系统依赖
install:
	@echo "安装系统依赖..."
	@if command -v apt-get >/dev/null 2>&1; then \
		sudo apt-get update && sudo apt-get install -y snapd debootstrap qemu-user-static lxc lxc-templates; \
	elif command -v yum >/dev/null 2>&1; then \
		sudo yum install -y snapd debootstrap qemu-user-static lxc lxc-templates; \
	else \
		echo "不支持的包管理器，请手动安装依赖"; \
	fi
	@echo "安装distrobuilder..."
	sudo snap install distrobuilder --classic
	@echo "依赖安装完成"

# 下载Go依赖
deps:
	@echo "下载Go依赖..."
	go mod download
	go mod tidy
	@echo "依赖下载完成"

# 代码检查
lint:
	@echo "运行代码检查..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint未安装，跳过代码检查"; \
		echo "安装命令: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# 代码格式化
format:
	@echo "格式化代码..."
	go fmt ./...
	@echo "代码格式化完成"

# 运行构建程序
run: build
	@echo "运行LXC镜像构建器..."
	@if [ ! -f "configs/images.yaml" ]; then \
		echo "错误: 配置文件 configs/images.yaml 不存在"; \
		exit 1; \
	fi
	@if [ ! -d "templates" ]; then \
		echo "错误: 模板目录 templates 不存在"; \
		exit 1; \
	fi
	@if ! command -v distrobuilder >/dev/null 2>&1; then \
		echo "错误: distrobuilder 未安装"; \
		echo "请运行: make install"; \
		exit 1; \
	fi
	mkdir -p output
	./lxc-builder configs/images.yaml output 3

# 使用Docker构建（可选）
docker:
	@echo "使用Docker构建..."
	docker build -t lxc-builder .
	docker run --rm -v $(PWD)/output:/app/output lxc-builder

# 快速开始
quickstart: deps install build
	@echo "快速开始设置完成"
	@echo "运行 'make run' 开始构建镜像"

# 开发环境设置
dev: deps format lint
	@echo "开发环境设置完成"

# 发布准备
release: clean deps format lint test build
	@echo "发布准备完成"
	@echo "构建文件: lxc-builder"
	@echo "配置文件: configs/images.yaml"
