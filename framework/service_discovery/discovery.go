package service_discovery

import (
	"context"
	"time"
)

// ServiceInfo 服务信息
type ServiceInfo struct {
	// 服务名称
	Name string `json:"name"`
	// 服务ID（唯一标识）
	ID string `json:"id"`
	// 服务地址（IP:Port）
	Address string `json:"address"`
	// 服务元数据
	Metadata map[string]string `json:"metadata"`
	// 服务标签
	Tags []string `json:"tags"`
	// 服务权重（用于负载均衡）
	Weight int `json:"weight"`
	// 服务状态
	Status ServiceStatus `json:"status"`
	// 最后更新时间
	LastUpdate time.Time `json:"last_update"`
}

// ServiceStatus 服务状态
type ServiceStatus string

const (
	// ServiceStatusHealthy 健康状态
	ServiceStatusHealthy ServiceStatus = "healthy"
	// ServiceStatusUnhealthy 不健康状态
	ServiceStatusUnhealthy ServiceStatus = "unhealthy"
	// ServiceStatusUnknown 未知状态
	ServiceStatusUnknown ServiceStatus = "unknown"
)

// Discovery 服务发现接口
type Discovery interface {
	// Register 注册服务
	Register(ctx context.Context, service *ServiceInfo, ttl time.Duration) error
	// Deregister 注销服务
	Deregister(ctx context.Context, serviceID string) error
	// Discover 发现服务
	Discover(ctx context.Context, serviceName string) ([]*ServiceInfo, error)
	// Watch 监听服务变化
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInfo, error)
	// HealthCheck 健康检查
	HealthCheck(ctx context.Context, serviceID string) error
	// Close 关闭发现客户端
	Close() error
}

// Registry 服务注册接口（扩展Discovery）
type Registry interface {
	Discovery
	// KeepAlive 保持服务活跃（续约）
	KeepAlive(ctx context.Context, serviceID string) error
	// ListServices 列出所有服务
	ListServices(ctx context.Context) ([]*ServiceInfo, error)
}

// Config 服务发现配置
type Config struct {
	// 注册中心地址（如etcd endpoints）
	Endpoints []string `json:"endpoints"`
	// 用户名（如果需要认证）
	Username string `json:"username"`
	// 密码（如果需要认证）
	Password string `json:"password"`
	// 连接超时
	DialTimeout time.Duration `json:"dial_timeout"`
	// 请求超时
	RequestTimeout time.Duration `json:"request_timeout"`
	// 租约TTL（秒）
	LeaseTTL int64 `json:"lease_ttl"`
	// 前缀（用于etcd key前缀）
	Prefix string `json:"prefix"`
	// 是否启用TLS
	EnableTLS bool `json:"enable_tls"`
	// TLS证书文件
	CertFile string `json:"cert_file"`
	// TLS密钥文件
	KeyFile string `json:"key_file"`
	// CA证书文件
	CAFile string `json:"ca_file"`
}

// NewServiceInfo 创建新的服务信息
func NewServiceInfo(name, id, address string) *ServiceInfo {
	return &ServiceInfo{
		Name:     name,
		ID:       id,
		Address:  address,
		Metadata: make(map[string]string),
		Tags:     []string{},
		Weight:   1,
		Status:   ServiceStatusHealthy,
	}
}

// AddMetadata 设置元数据
func (si *ServiceInfo) AddMetadata(key, value string) *ServiceInfo {
	if si.Metadata == nil {
		si.Metadata = make(map[string]string)
	}
	si.Metadata[key] = value
	return si
}

// AddTag 添加标签
func (si *ServiceInfo) AddTag(tag string) *ServiceInfo {
	si.Tags = append(si.Tags, tag)
	return si
}

// SetWeight 设置权重
func (si *ServiceInfo) SetWeight(weight int) *ServiceInfo {
	si.Weight = weight
	return si
}
