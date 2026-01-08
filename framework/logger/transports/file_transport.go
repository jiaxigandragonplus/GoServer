package transports

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileConfig 文件传输配置
type FileConfig struct {
	// FilePath 日志文件路径
	FilePath string `json:"file_path" yaml:"file_path"`
	// MaxSize 日志文件最大大小（MB），默认100M
	MaxSize int `json:"max_size" yaml:"max_size"`
	// MaxBackups 最大备份文件数
	MaxBackups int `json:"max_backups" yaml:"max_backups"`
	// MaxAge 最大保存天数
	MaxAge int `json:"max_age" yaml:"max_age"`
	// Compress 是否压缩备份文件
	Compress bool `json:"compress" yaml:"compress"`
	// RotationInterval 时间轮转间隔，如"24h"、"1h"、"30m"，为空表示不按时间轮转
	RotationInterval string `json:"rotation_interval" yaml:"rotation_interval"`
	// RotationTime 每天轮转的特定时间，格式"15:04"，如"00:00"表示每天午夜轮转
	RotationTime string `json:"rotation_time" yaml:"rotation_time"`
}

// 创建一个默认的文件传输配置
func NewFileConfig(fileName string) *FileConfig {
	return &FileConfig{
		FilePath:         fmt.Sprintf("logs/%s.log", fileName),
		MaxSize:          100,
		MaxBackups:       10,
		MaxAge:           30,
		Compress:         false,
		RotationInterval: "",
		RotationTime:     "",
	}
}

// fileTransport 文件传输实现
type fileTransport struct {
	*lumberjack.Logger
	config       *FileConfig
	rotationDone chan struct{}
	stopChan     chan struct{}
	mu           sync.Mutex
}

// FileTransportFactory 文件传输工厂
type FileTransportFactory struct {
	config *FileConfig
}

// NewFileTransportFactory 创建文件传输工厂
func NewFileTransportFactory(config *FileConfig) *FileTransportFactory {
	return &FileTransportFactory{
		config: config,
	}
}

// Create 创建文件传输实例
func (f *FileTransportFactory) Create() (Transport, error) {
	if f.config.FilePath == "" {
		return nil, fmt.Errorf("file path is required for file transport")
	}

	// 确保目录存在
	dir := filepath.Dir(f.config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory failed: %w", err)
	}

	// 设置默认MaxSize为100M
	maxSize := f.config.MaxSize
	if maxSize <= 0 {
		maxSize = 100
	}

	lumberjackLogger := &lumberjack.Logger{
		Filename:   f.config.FilePath,
		MaxSize:    maxSize,             // MB
		MaxBackups: f.config.MaxBackups, // 备份文件数
		MaxAge:     f.config.MaxAge,     // 天数
		Compress:   f.config.Compress,   // 是否压缩
		LocalTime:  true,                // 使用本地时间
	}

	transport := &fileTransport{
		Logger:       lumberjackLogger,
		config:       f.config,
		rotationDone: make(chan struct{}),
		stopChan:     make(chan struct{}),
	}

	// 启动定时轮转
	if err := transport.startRotation(); err != nil {
		return nil, err
	}

	return transport, nil
}

// Name 返回传输名称
func (f *FileTransportFactory) Name() string {
	return "file"
}

// startRotation 启动定时轮转
func (t *fileTransport) startRotation() error {
	// 解析时间间隔
	interval, err := t.parseRotationInterval()
	if err != nil {
		return fmt.Errorf("parse rotation interval failed: %w", err)
	}

	if interval <= 0 {
		// 没有设置时间轮转，只按大小轮转
		return nil
	}

	// 启动goroutine进行定时轮转
	go t.rotationLoop(interval)
	return nil
}

// parseRotationInterval 解析轮转时间间隔
func (t *fileTransport) parseRotationInterval() (time.Duration, error) {
	if t.config.RotationInterval == "" {
		return 0, nil
	}

	// 解析时间间隔字符串，如"24h"、"1h"、"30m"
	duration, err := time.ParseDuration(t.config.RotationInterval)
	if err != nil {
		return 0, fmt.Errorf("invalid rotation interval format: %s, expected format like '24h', '1h', '30m'", t.config.RotationInterval)
	}

	return duration, nil
}

// rotationLoop 定时轮转循环
func (t *fileTransport) rotationLoop(interval time.Duration) {
	// 计算第一次轮转的时间
	var nextRotation time.Time
	if t.config.RotationTime != "" {
		// 如果设置了具体时间，计算到下一个该时间的间隔
		nextRotation = t.calculateNextRotationTime()
	} else {
		// 否则按固定间隔轮转
		nextRotation = time.Now().Add(interval)
	}

	ticker := time.NewTicker(time.Minute) // 每分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-t.stopChan:
			close(t.rotationDone)
			return
		case now := <-ticker.C:
			if now.After(nextRotation) {
				t.mu.Lock()
				if err := t.Rotate(); err != nil {
					// 记录错误，但继续运行
					fmt.Fprintf(os.Stderr, "log rotation failed: %v\n", err)
				}
				t.mu.Unlock()

				// 计算下一次轮转时间
				if t.config.RotationTime != "" {
					nextRotation = t.calculateNextRotationTime()
				} else {
					nextRotation = time.Now().Add(interval)
				}
			}
		}
	}
}

// calculateNextRotationTime 计算下一次轮转时间
func (t *fileTransport) calculateNextRotationTime() time.Time {
	now := time.Now()

	if t.config.RotationTime == "" {
		// 如果没有设置具体时间，返回现在（不应该发生）
		return now
	}

	// 解析时间，格式为"15:04"
	parts := strings.Split(t.config.RotationTime, ":")
	if len(parts) != 2 {
		// 格式错误，使用默认时间
		return now.Add(24 * time.Hour)
	}

	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		// 解析失败，使用默认时间
		return now.Add(24 * time.Hour)
	}

	// 构造今天的目标时间
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

	// 如果今天的目标时间已经过去，则使用明天
	if now.After(target) {
		target = target.Add(24 * time.Hour)
	}

	return target
}

// Write 实现io.Writer接口
func (t *fileTransport) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Logger.Write(p)
}

// Sync 实现zapcore.WriteSyncer接口
func (t *fileTransport) Sync() error {
	// lumberjack.Logger没有Sync方法，返回nil
	return nil
}

// Close 关闭传输，停止定时轮转
func (t *fileTransport) Close() error {
	close(t.stopChan)
	<-t.rotationDone
	return t.Logger.Close()
}

// init 初始化时注册文件传输
func init() {
	// 注册文件传输工厂
	Register(&FileTransportFactory{
		config: &FileConfig{
			MaxSize:    100,
			MaxBackups: 10,
			MaxAge:     30,
			Compress:   true,
		},
	})
}
