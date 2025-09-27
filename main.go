package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ImageConfig 定义镜像构建配置
type ImageConfig struct {
	Name        string            `yaml:"name"`
	Distro      string            `yaml:"distro"`
	Release     string            `yaml:"release"`
	Arch        string            `yaml:"arch"`
	Variant     string            `yaml:"variant"`
	Packages    []string          `yaml:"packages"`
	Files       map[string]string `yaml:"files"`
	Actions     []Action          `yaml:"actions"`
}

// Action 定义构建动作
type Action struct {
	Type    string            `yaml:"type"`
	Action  string            `yaml:"action"`
	Options map[string]string `yaml:"options"`
}

// BuildJob 定义构建任务
type BuildJob struct {
	Config ImageConfig
	Output string
	Error  error
}

// Builder 镜像构建器
type Builder struct {
	configs    []ImageConfig
	outputDir  string
	concurrent int
	logger     *logrus.Logger
}

// NewBuilder 创建新的构建器
func NewBuilder(configFile, outputDir string, concurrent int) (*Builder, error) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 读取配置文件
	configs, err := loadConfigs(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configs: %w", err)
	}

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Builder{
		configs:    configs,
		outputDir:  outputDir,
		concurrent: concurrent,
		logger:     logger,
	}, nil
}

// loadConfigs 加载配置文件
func loadConfigs(configFile string) ([]ImageConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var configs []ImageConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, err
	}

	return configs, nil
}

// BuildAll 构建所有镜像
func (b *Builder) BuildAll(ctx context.Context) error {
	b.logger.Info("Starting image build process...")

	// 创建任务通道
	jobs := make(chan BuildJob, len(b.configs))
	results := make(chan BuildJob, len(b.configs))

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < b.concurrent; i++ {
		wg.Add(1)
		go b.worker(ctx, i, jobs, results, &wg)
	}

	// 发送任务
	go func() {
		defer close(jobs)
		for _, config := range b.configs {
			select {
			case jobs <- BuildJob{Config: config}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待所有工作协程完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var errors []error
	for result := range results {
		if result.Error != nil {
			b.logger.Errorf("Failed to build %s: %v", result.Config.Name, result.Error)
			errors = append(errors, result.Error)
		} else {
			b.logger.Infof("Successfully built %s: %s", result.Config.Name, result.Output)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("build failed with %d errors", len(errors))
	}

	b.logger.Info("All images built successfully!")
	return nil
}

// worker 工作协程
func (b *Builder) worker(ctx context.Context, id int, jobs <-chan BuildJob, results chan<- BuildJob, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b.logger.Infof("Worker %d: Building %s", id, job.Config.Name)
		
		output, err := b.buildImage(ctx, job.Config)
		job.Output = output
		job.Error = err

		select {
		case results <- job:
		case <-ctx.Done():
			return
		}
	}
}

// buildImage 构建单个镜像
func (b *Builder) buildImage(ctx context.Context, config ImageConfig) (string, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "distrobuilder-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 生成distrobuilder配置
	configFile := filepath.Join(tempDir, "config.yaml")
	if err := b.generateDistrobuilderConfig(config, configFile); err != nil {
		return "", fmt.Errorf("failed to generate distrobuilder config: %w", err)
	}

	// 执行distrobuilder
	outputFile := filepath.Join(b.outputDir, fmt.Sprintf("%s.tar.gz", config.Name))
	cmd := exec.CommandContext(ctx, "distrobuilder", "build-lxc", configFile, outputFile)
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("distrobuilder failed: %w\nOutput: %s", err, string(output))
	}

	return outputFile, nil
}

// generateDistrobuilderConfig 生成distrobuilder配置文件
func (b *Builder) generateDistrobuilderConfig(config ImageConfig, outputFile string) error {
	// 基础配置
	imageConfig := map[string]interface{}{
		"distribution": config.Distro,
		"release":      config.Release,
		"architecture": config.Arch,
	}
	
	// 添加变体配置
	if config.Variant != "" {
		imageConfig["variant"] = config.Variant
	}
	
	distrobuilderConfig := map[string]interface{}{
		"image": imageConfig,
	}
	
	// 根据发行版设置不同的源和包管理器
	switch config.Distro {
	case "ubuntu":
		distrobuilderConfig["source"] = map[string]interface{}{
			"downloader": "debootstrap",
			"url":        "http://archive.ubuntu.com/ubuntu",
		}
		distrobuilderConfig["packages"] = map[string]interface{}{
			"manager": "apt",
			"update":  true,
			"install": append(config.Packages, "openssh-server", "sudo"),
		}
	case "debian":
		distrobuilderConfig["source"] = map[string]interface{}{
			"downloader": "debootstrap",
			"url":        "http://deb.debian.org/debian",
		}
		distrobuilderConfig["packages"] = map[string]interface{}{
			"manager": "apt",
			"update":  true,
			"install": append(config.Packages, "openssh-server", "sudo"),
		}
	case "centos":
		distrobuilderConfig["source"] = map[string]interface{}{
			"downloader": "yum",
			"url":        "http://mirror.centos.org/centos",
		}
		distrobuilderConfig["packages"] = map[string]interface{}{
			"manager": "yum",
			"update":  true,
			"install": append(config.Packages, "openssh-server", "sudo"),
		}
	default:
		// 默认使用debootstrap
		distrobuilderConfig["source"] = map[string]interface{}{
			"downloader": "debootstrap",
			"url":        "http://archive.ubuntu.com/ubuntu",
		}
		distrobuilderConfig["packages"] = map[string]interface{}{
			"manager": "apt",
			"update":  true,
			"install": append(config.Packages, "openssh-server", "sudo"),
		}
	}
	
	// 添加SSH配置文件
	distrobuilderConfig["files"] = []map[string]interface{}{
		{
			"path":    "/etc/ssh/sshd_config",
			"content": generateSSHConfig(),
		},
		{
			"path":    "/etc/systemd/system/ssh.service",
			"content": generateSSHService(),
		},
	}
	
	// 添加SSH配置动作
	actions := []map[string]interface{}{
		{
			"trigger": "post-files",
			"action":  "run",
			"command": "systemctl enable ssh",
		},
		{
			"trigger": "post-files",
			"action":  "run",
			"command": "echo 'root:password' | chpasswd",
		},
		{
			"trigger": "post-files",
			"action":  "run",
			"command": "sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config",
		},
		{
			"trigger": "post-files",
			"action":  "run",
			"command": "sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config",
		},
	}
	
	// 根据发行版添加特定的SSH配置命令
	if config.Distro == "centos" {
		actions = append(actions, map[string]interface{}{
			"trigger": "post-files",
			"action":  "run",
			"command": "systemctl enable sshd",
		})
	}
	
	distrobuilderConfig["actions"] = actions

	// 添加自定义文件
	for path, content := range config.Files {
		distrobuilderConfig["files"] = append(distrobuilderConfig["files"].([]map[string]interface{}), map[string]interface{}{
			"path":    path,
			"content": content,
		})
	}

	// 添加自定义动作
	for _, action := range config.Actions {
		distrobuilderConfig["actions"] = append(distrobuilderConfig["actions"].([]map[string]interface{}), map[string]interface{}{
			"trigger": action.Type,
			"action":  action.Action,
			"command": action.Options["command"],
		})
	}

	// 写入配置文件
	data, err := yaml.Marshal(distrobuilderConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(outputFile, data, 0644)
}

// generateSSHConfig 生成SSH配置文件内容
func generateSSHConfig() string {
	return `# SSH Server Configuration
Port 22
Protocol 2
HostKey /etc/ssh/ssh_host_rsa_key
HostKey /etc/ssh/ssh_host_ecdsa_key
HostKey /etc/ssh/ssh_host_ed25519_key

# Logging
SyslogFacility AUTH
LogLevel INFO

# Authentication
LoginGraceTime 120
PermitRootLogin yes
StrictModes yes
PubkeyAuthentication yes
PasswordAuthentication yes
PermitEmptyPasswords no
ChallengeResponseAuthentication no

# Network
X11Forwarding yes
X11DisplayOffset 10
PrintMotd no
PrintLastLog yes
TCPKeepAlive yes
UsePrivilegeSeparation yes

# Subsystem
Subsystem sftp /usr/lib/openssh/sftp-server
`
}

// generateSSHService 生成SSH服务配置
func generateSSHService() string {
	return `[Unit]
Description=OpenBSD Secure Shell server
After=network.target auditd.service
ConditionPathExists=!/etc/ssh/sshd_not_to_be_run

[Service]
EnvironmentFile=-/etc/default/ssh
ExecStartPre=/usr/sbin/sshd -t
ExecStart=/usr/sbin/sshd -D $SSHD_OPTS
ExecReload=/usr/sbin/sshd -t
ExecReload=/bin/kill -HUP $MAINPID
KillMode=process
Restart=on-failure
RestartPreventExitStatus=255
Type=notify
RuntimeDirectory=sshd
RuntimeDirectoryMode=0755

[Install]
WantedBy=multi-user.target
`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <config.yaml> [output-dir] [concurrent-jobs]")
	}

	configFile := os.Args[1]
	outputDir := "output"
	concurrent := 3

	if len(os.Args) > 2 {
		outputDir = os.Args[2]
	}
	if len(os.Args) > 3 {
		if c, err := fmt.Sscanf(os.Args[3], "%d", &concurrent); err != nil || c != 1 {
			log.Fatal("Invalid concurrent jobs number")
		}
	}

	// 检查distrobuilder是否可用
	if _, err := exec.LookPath("distrobuilder"); err != nil {
		log.Fatal("distrobuilder not found in PATH. Please install it first.")
	}

	// 创建构建器
	builder, err := NewBuilder(configFile, outputDir, concurrent)
	if err != nil {
		log.Fatalf("Failed to create builder: %v", err)
	}

	// 开始构建
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := builder.BuildAll(ctx); err != nil {
		log.Fatalf("Build failed: %v", err)
	}

	fmt.Println("All images built successfully!")
}
