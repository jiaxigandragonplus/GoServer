package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GooLuck/GoServer/framework/service_discovery"
)

// TestServiceDiscovery 测试服务发现基本功能
func TestServiceDiscovery(t *testing.T) {
	// 注意：这个测试需要etcd服务器运行
	// 在实际测试中，应该使用嵌入式etcd或mock
	//t.Skip("Skipping test that requires etcd server")

	config := &service_discovery.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 2 * time.Second,
		LeaseTTL:    5,
		Prefix:      "/test-services",
	}

	discovery, err := service_discovery.NewEtcdDiscovery(config)
	if err != nil {
		t.Fatalf("Failed to create discovery: %v", err)
	}
	defer discovery.Close()

	ctx := context.Background()

	// 测试服务注册
	service := service_discovery.NewServiceInfo("test-service", "test-1", "localhost:9999")
	err = discovery.Register(ctx, service, 5*time.Second)
	if err != nil {
		t.Errorf("Failed to register service: %v", err)
	}

	// 测试服务发现
	services, err := discovery.Discover(ctx, "test-service")
	if err != nil {
		t.Errorf("Failed to discover services: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}

	if services[0].ID != "test-1" {
		t.Errorf("Expected service ID 'test-1', got '%s'", services[0].ID)
	}

	// 测试服务注销
	err = discovery.Deregister(ctx, "test-1")
	if err != nil {
		t.Errorf("Failed to deregister service: %v", err)
	}
}

// TestServiceInfoBuilder 测试ServiceInfo构建器
func TestServiceInfoBuilder(t *testing.T) {
	service := service_discovery.NewServiceInfo("my-service", "id-123", "localhost:8080").
		AddMetadata("version", "1.0.0").
		AddTag("http").
		AddTag("rest").
		SetWeight(5)

	if service.Name != "my-service" {
		t.Errorf("Expected name 'my-service', got '%s'", service.Name)
	}

	if service.ID != "id-123" {
		t.Errorf("Expected ID 'id-123', got '%s'", service.ID)
	}

	if service.Address != "localhost:8080" {
		t.Errorf("Expected address 'localhost:8080', got '%s'", service.Address)
	}

	if service.Metadata["version"] != "1.0.0" {
		t.Errorf("Expected metadata version '1.0.0', got '%s'", service.Metadata["version"])
	}

	if len(service.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(service.Tags))
	}

	if service.Weight != 5 {
		t.Errorf("Expected weight 5, got %d", service.Weight)
	}
}

// TestHealthChecker 测试健康检查器
func TestHealthChecker(t *testing.T) {
	// 创建HTTP健康检查器
	checker := service_discovery.NewHTTPHealthChecker("/health", 1*time.Second)
	if checker.Type() != "http" {
		t.Errorf("Expected checker type 'http', got '%s'", checker.Type())
	}

	// 创建TCP健康检查器
	tcpChecker := service_discovery.NewTCPHealthChecker(1 * time.Second)
	if tcpChecker.Type() != "tcp" {
		t.Errorf("Expected checker type 'tcp', got '%s'", tcpChecker.Type())
	}
}

// TestConcurrentRegistration 测试并发注册
func TestConcurrentRegistration(t *testing.T) {
	//t.Skip("Skipping concurrent test that requires etcd server")

	config := &service_discovery.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 2 * time.Second,
		Prefix:      "/concurrent-services",
	}

	discovery, err := service_discovery.NewEtcdDiscovery(config)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}
	defer discovery.Close()

	ctx := context.Background()

	// 并发注册10个服务
	const numServices = 10
	errCh := make(chan error, numServices)

	for i := 0; i < numServices; i++ {
		go func(id int) {
			service := service_discovery.NewServiceInfo("concurrent-service", fmt.Sprintf("concurrent-%d", id), fmt.Sprintf("localhost:%d", 9000+id))
			err := discovery.Register(ctx, service, 5*time.Second)
			errCh <- err
		}(i)
	}

	// 收集错误
	for i := 0; i < numServices; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent registration failed: %v", err)
		}
	}
}
