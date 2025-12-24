package service_discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// etcdDiscovery 基于etcd的服务发现实现
type etcdDiscovery struct {
	client     *clientv3.Client
	config     *Config
	logger     *zap.Logger
	leaseID    clientv3.LeaseID
	registered map[string]bool // 已注册的服务ID
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	watchCh    map[string]chan []*ServiceInfo // 监听通道
	watchMu    sync.RWMutex
}

// NewEtcdDiscovery 创建新的etcd服务发现实例
func NewEtcdDiscovery(config *Config) (Registry, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if len(config.Endpoints) == 0 {
		config.Endpoints = []string{"localhost:2379"}
	}

	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}

	if config.RequestTimeout == 0 {
		config.RequestTimeout = 3 * time.Second
	}

	if config.LeaseTTL == 0 {
		config.LeaseTTL = 10 // 默认10秒
	}

	if config.Prefix == "" {
		config.Prefix = "/services"
	}

	// 创建etcd客户端配置
	etcdConfig := clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: config.DialTimeout,
	}

	if config.Username != "" && config.Password != "" {
		etcdConfig.Username = config.Username
		etcdConfig.Password = config.Password
	}

	// 创建etcd客户端
	client, err := clientv3.New(etcdConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	discovery := &etcdDiscovery{
		client:     client,
		config:     config,
		logger:     zap.L().Named("etcd-discovery"),
		registered: make(map[string]bool),
		ctx:        ctx,
		cancel:     cancel,
		watchCh:    make(map[string]chan []*ServiceInfo),
	}

	return discovery, nil
}

// Register 注册服务到etcd
func (ed *etcdDiscovery) Register(ctx context.Context, service *ServiceInfo, ttl time.Duration) error {
	if service == nil {
		return errors.New("service cannot be nil")
	}

	if service.ID == "" {
		return errors.New("service ID cannot be empty")
	}

	if service.Name == "" {
		return errors.New("service name cannot be empty")
	}

	// 使用配置的超时时间创建上下文
	reqCtx, cancel := context.WithTimeout(ctx, ed.config.RequestTimeout)
	defer cancel()

	// 序列化服务信息
	serviceData, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service info: %w", err)
	}

	// 创建租约
	leaseTTL := int64(ttl.Seconds())
	if leaseTTL == 0 {
		leaseTTL = ed.config.LeaseTTL
	}

	leaseResp, err := ed.client.Grant(reqCtx, leaseTTL)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}

	// 构建key
	key := ed.buildServiceKey(service.Name, service.ID)

	// 将服务信息写入etcd
	_, err = ed.client.Put(reqCtx, key, string(serviceData), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return fmt.Errorf("failed to put service to etcd: %w", err)
	}

	// 保存租约ID和服务ID
	ed.mu.Lock()
	ed.leaseID = leaseResp.ID
	ed.registered[service.ID] = true
	ed.mu.Unlock()

	ed.logger.Info("service registered",
		zap.String("service", service.Name),
		zap.String("id", service.ID),
		zap.String("address", service.Address),
		zap.Int64("lease_id", int64(leaseResp.ID)),
	)

	return nil
}

// Deregister 从etcd注销服务
func (ed *etcdDiscovery) Deregister(ctx context.Context, serviceID string) error {
	// 使用配置的超时时间创建上下文
	reqCtx, cancel := context.WithTimeout(ctx, ed.config.RequestTimeout)
	defer cancel()

	// 为了从etcd删除服务，我们需要知道服务名称
	// 这里简化处理：遍历所有服务找到对应的服务
	// 实际应用中，应该维护服务ID到服务名称的映射
	services, err := ed.ListServices(reqCtx)
	if err != nil {
		return fmt.Errorf("failed to list services for deregistration: %w", err)
	}

	var serviceToDelete *ServiceInfo
	for _, svc := range services {
		if svc.ID == serviceID {
			serviceToDelete = svc
			break
		}
	}

	if serviceToDelete == nil {
		// 服务不存在，只从本地注册表中移除
		ed.mu.Lock()
		delete(ed.registered, serviceID)
		ed.mu.Unlock()
		return nil
	}

	// 构建key并从etcd删除
	key := ed.buildServiceKey(serviceToDelete.Name, serviceID)
	_, err = ed.client.Delete(reqCtx, key)
	if err != nil {
		return fmt.Errorf("failed to delete service from etcd: %w", err)
	}

	// 从本地注册表中移除
	ed.mu.Lock()
	delete(ed.registered, serviceID)
	ed.mu.Unlock()

	ed.logger.Info("service deregistered",
		zap.String("id", serviceID),
		zap.String("name", serviceToDelete.Name),
	)

	return nil
}

// Discover 从etcd发现服务
func (ed *etcdDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInfo, error) {
	if serviceName == "" {
		return nil, errors.New("service name cannot be empty")
	}

	// 使用配置的超时时间创建上下文
	reqCtx, cancel := context.WithTimeout(ctx, ed.config.RequestTimeout)
	defer cancel()

	// 构建服务前缀
	prefix := ed.buildServicePrefix(serviceName)

	// 从etcd获取所有服务实例
	resp, err := ed.client.Get(reqCtx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get services from etcd: %w", err)
	}

	services := make([]*ServiceInfo, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var service ServiceInfo
		if err := json.Unmarshal(kv.Value, &service); err != nil {
			ed.logger.Warn("failed to unmarshal service info",
				zap.String("key", string(kv.Key)),
				zap.Error(err),
			)
			continue
		}
		services = append(services, &service)
	}

	return services, nil
}

// Watch 监听服务变化
func (ed *etcdDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInfo, error) {
	if serviceName == "" {
		return nil, errors.New("service name cannot be empty")
	}

	// 创建监听通道
	ch := make(chan []*ServiceInfo, 10)

	ed.watchMu.Lock()
	ed.watchCh[serviceName] = ch
	ed.watchMu.Unlock()

	// 启动后台监听
	go ed.watchService(serviceName, ch)

	return ch, nil
}

// HealthCheck 健康检查
func (ed *etcdDiscovery) HealthCheck(ctx context.Context, serviceID string) error {
	// 使用配置的超时时间创建上下文
	reqCtx, cancel := context.WithTimeout(ctx, ed.config.RequestTimeout)
	defer cancel()

	// 简化实现：检查etcd连接是否正常
	_, err := ed.client.Get(reqCtx, "/health", clientv3.WithKeysOnly())
	if err != nil {
		return fmt.Errorf("etcd health check failed: %w", err)
	}
	return nil
}

// KeepAlive 保持服务活跃（续约）
func (ed *etcdDiscovery) KeepAlive(ctx context.Context, serviceID string) error {
	ed.mu.RLock()
	leaseID := ed.leaseID
	ed.mu.RUnlock()

	if leaseID == 0 {
		return errors.New("no active lease found")
	}

	// 使用配置的超时时间创建上下文
	reqCtx, cancel := context.WithTimeout(ctx, ed.config.RequestTimeout)
	defer cancel()

	// 续约租约
	_, err := ed.client.KeepAliveOnce(reqCtx, leaseID)
	if err != nil {
		return fmt.Errorf("failed to keep alive: %w", err)
	}

	return nil
}

// ListServices 列出所有服务
func (ed *etcdDiscovery) ListServices(ctx context.Context) ([]*ServiceInfo, error) {
	// 使用配置的超时时间创建上下文
	reqCtx, cancel := context.WithTimeout(ctx, ed.config.RequestTimeout)
	defer cancel()

	// 获取所有服务
	resp, err := ed.client.Get(reqCtx, ed.config.Prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	services := make([]*ServiceInfo, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var service ServiceInfo
		if err := json.Unmarshal(kv.Value, &service); err != nil {
			ed.logger.Warn("failed to unmarshal service info",
				zap.String("key", string(kv.Key)),
				zap.Error(err),
			)
			continue
		}
		services = append(services, &service)
	}

	return services, nil
}

// Close 关闭etcd客户端
func (ed *etcdDiscovery) Close() error {
	ed.cancel()
	ed.watchMu.Lock()
	for _, ch := range ed.watchCh {
		close(ch)
	}
	ed.watchCh = make(map[string]chan []*ServiceInfo)
	ed.watchMu.Unlock()

	return ed.client.Close()
}

// buildServiceKey 构建服务key
func (ed *etcdDiscovery) buildServiceKey(serviceName, serviceID string) string {
	return path.Join(ed.config.Prefix, serviceName, serviceID)
}

// buildServicePrefix 构建服务前缀
func (ed *etcdDiscovery) buildServicePrefix(serviceName string) string {
	return path.Join(ed.config.Prefix, serviceName) + "/"
}

// watchService 监听服务变化
func (ed *etcdDiscovery) watchService(serviceName string, ch chan<- []*ServiceInfo) {
	prefix := ed.buildServicePrefix(serviceName)
	watchChan := ed.client.Watch(ed.ctx, prefix, clientv3.WithPrefix())

	for {
		select {
		case <-ed.ctx.Done():
			return
		case resp := <-watchChan:
			if resp.Err() != nil {
				ed.logger.Error("watch error",
					zap.String("service", serviceName),
					zap.Error(resp.Err()),
				)
				continue
			}

			// 获取当前所有服务
			ctx, cancel := context.WithTimeout(context.Background(), ed.config.RequestTimeout)
			services, err := ed.Discover(ctx, serviceName)
			cancel()

			if err != nil {
				ed.logger.Error("failed to discover services after watch event",
					zap.String("service", serviceName),
					zap.Error(err),
				)
				continue
			}

			// 发送更新
			select {
			case ch <- services:
			default:
				ed.logger.Warn("watch channel is full, dropping update",
					zap.String("service", serviceName),
				)
			}
		}
	}
}

// 辅助函数：从key中提取服务名称
func extractServiceNameFromKey(key, prefix string) string {
	relativeKey := strings.TrimPrefix(key, prefix+"/")
	parts := strings.Split(relativeKey, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
