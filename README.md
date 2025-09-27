# LXC Images Builder with SSH Support

这是一个使用Go语言和distrobuilder构建LXC镜像的项目，支持多任务并行构建，所有镜像都预装了SSH服务器并配置了root密码登录。

## 功能特性

- 🚀 **多任务并行构建**: 使用Go协程实现高效的并行镜像构建
- 🔧 **SSH预配置**: 所有镜像都预装SSH服务器，支持root密码登录
- 📦 **多发行版支持**: 支持Ubuntu、Debian、CentOS、Alpine等主流发行版
- 🔄 **GitHub Actions集成**: 自动化构建、测试和发布流程
- ⚡ **可配置并发**: 支持自定义并发构建任务数量

## 项目结构

```
lxc-images-build/
├── main.go                 # Go构建程序主文件
├── go.mod                  # Go模块依赖
├── configs/
│   └── images.yaml         # 镜像构建配置文件
├── templates/              # distrobuilder配置模板
│   ├── ubuntu.yaml         # Ubuntu模板
│   ├── debian.yaml         # Debian模板
│   └── centos.yaml         # CentOS模板
├── .github/
│   └── workflows/
│       └── build-images.yml # GitHub Actions工作流
└── README.md               # 项目文档
```

## 快速开始

### 1. 环境要求

- Go 1.21+
- distrobuilder
- LXC/LXD
- 构建工具: debootstrap, qemu-user-static

### 2. 安装依赖

#### Ubuntu/Debian:
```bash
# 安装distrobuilder
sudo snap install distrobuilder --classic

# 安装构建工具
sudo apt-get update
sudo apt-get install -y debootstrap qemu-user-static lxc lxc-templates
```

#### CentOS/RHEL:
```bash
# 安装distrobuilder
sudo snap install distrobuilder --classic

# 安装构建工具
sudo yum install -y debootstrap qemu-user-static lxc lxc-templates
```

### 3. 构建镜像

```bash
# 克隆项目
git clone <your-repo-url>
cd lxc-images-build

# 下载Go依赖
go mod download

# 构建程序
go build -o lxc-builder main.go

# 运行构建（使用默认配置）
./lxc-builder configs/images.yaml output 3

# 参数说明:
# configs/images.yaml - 配置文件路径
# output - 输出目录
# 3 - 并发构建任务数
```

**注意**: 确保 `templates/` 目录存在且包含所需的模板文件。

### 4. 使用构建的镜像

```bash
# 导入镜像
sudo lxc image import output/ubuntu-22.04-ssh.tar.gz --alias ubuntu-ssh

# 创建容器
sudo lxc launch ubuntu-ssh my-container

# 获取容器IP
sudo lxc list

# SSH连接（默认密码: password）
ssh root@<container-ip>
```

## 配置文件说明

`configs/images.yaml` 文件定义了要构建的镜像配置：

```yaml
- name: ubuntu-22.04-ssh          # 镜像名称
  distro: ubuntu                  # 发行版
  release: jammy                  # 版本
  arch: amd64                     # 架构
  packages:                       # 额外安装的包
    - curl
    - wget
    - vim
  files:                          # 自定义文件
    /etc/motd: "Welcome message"
  actions:                        # 构建后执行的动作
    - type: post-files
      action: run
      options:
        command: "apt-get update"
```

### 支持的发行版

- **Ubuntu**: focal (20.04), jammy (22.04)
- **Debian**: bullseye (11), bookworm (12)
- **CentOS**: 8
- **Alpine**: 3.18

## GitHub Actions 自动化

项目包含完整的GitHub Actions工作流，支持：

### 触发条件
- 推送到main/master分支
- 配置文件或代码变更
- 手动触发（workflow_dispatch）

### 工作流程
1. **Setup**: 安装依赖，解析配置
2. **Build**: 并行构建所有镜像
3. **Test**: 测试镜像功能（SSH服务、密码登录）
4. **Release**: 创建GitHub Release并上传镜像

### 手动触发
在GitHub仓库页面，进入Actions标签，选择"Build LXC Images with SSH"工作流，点击"Run workflow"可以手动触发构建。

## 自定义配置

### 添加新的镜像

在 `configs/images.yaml` 中添加新的镜像配置：

```yaml
- name: my-custom-image
  distro: ubuntu
  release: jammy
  arch: amd64
  packages:
    - nginx
    - mysql-server
  files:
    /etc/nginx/nginx.conf: |
      # 自定义nginx配置
  actions:
    - type: post-files
      action: run
      options:
        command: "systemctl enable nginx"
```

### 修改SSH配置

SSH配置在 `main.go` 的 `generateSSHConfig()` 函数中定义，可以修改：

- 端口号
- 认证方式
- 登录限制
- 其他SSH选项

### 修改默认密码

在 `main.go` 的 `generateDistrobuilderConfig()` 函数中修改：

```go
{
    "trigger": "post-files",
    "action":  "run",
    "command": "echo 'root:your-new-password' | chpasswd",
}
```

## 安全注意事项

⚠️ **重要**: 默认配置使用弱密码 `password`，仅用于开发和测试环境。

生产环境使用前请务必：
1. 修改默认root密码
2. 禁用密码认证，使用SSH密钥
3. 配置防火墙规则
4. 定期更新系统和软件包

## 故障排除

### 常见问题

1. **distrobuilder未找到**
   ```bash
   sudo snap install distrobuilder --classic
   ```

2. **权限问题**
   ```bash
   sudo usermod -a -G lxd $USER
   newgrp lxd
   ```

3. **构建失败**
   - 检查网络连接
   - 确保有足够的磁盘空间
   - 查看详细错误日志

4. **SSH连接失败**
   - 检查容器是否启动
   - 确认SSH服务状态: `sudo lxc exec <container> -- systemctl status ssh`
   - 检查防火墙设置

### 调试模式

启用详细日志：
```bash
export DISTROBUILDER_DEBUG=1
./lxc-builder configs/images.yaml output 1
```

## 贡献

欢迎提交Issue和Pull Request！

1. Fork项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 创建Pull Request

## 许可证

MIT License

## 更新日志

### v1.0.0
- 初始版本
- 支持多发行版镜像构建
- SSH预配置
- GitHub Actions集成
- 并行构建支持
