package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/GooLuck/GoServer/framework/service_discovery"
)

func main() {
	// 1. 创建etcd服务发现配置
	config := &service_discovery.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
		LeaseTTL:    10, // 10秒租约
		Prefix:      "/services",
	}

	// 2. 创建etcd服务发现实例
	discovery, err := service_discovery.NewEtcdDiscovery(config)
	if err != nil {
		log.Fatalf("Failed to create discovery: %v", err)
	}
	defer discovery.Close()

	ctx := context.Background()

	// 3. 注册服务
	service1 := service_discovery.NewServiceInfo("user-service", "user-1", "localhost:8080").
		AddMetadata("version", "1.0.0").
		AddTag("http").
		AddTag("rest").
		SetWeight(1)

	err = discovery.Register(ctx, service1, 10*time.Second)
	if err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}
	fmt.Printf("Registered service: %s (ID: %s)\n", service1.Name, service1.ID)

	// 4. 注册另一个服务实例
	service2 := service_discovery.NewServiceInfo("user-service", "user-2", "localhost:8081").
		AddMetadata("version", "1.0.0").
		AddTag("http").
		AddTag("rest").
		SetWeight(2)

	err = discovery.Register(ctx, service2, 10*time.Second)
	if err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}
	fmt.Printf("Registered service: %s (ID: %s)\n", service2.Name, service2.ID)

	// 5. 发现服务
	services, err := discovery.Discover(ctx, "user-service")
	if err != nil {
		log.Fatalf("Failed to discover services: %v", err)
	}

	fmt.Printf("\nDiscovered %d services:\n", len(services))
	for i, svc := range services {
		fmt.Printf("  %d. ID: %s, Address: %s, Weight: %d, Tags: %v\n",
			i+1, svc.ID, svc.Address, svc.Weight, svc.Tags)
	}

	// 6. 监听服务变化
	watchCh, err := discovery.Watch(ctx, "user-service")
	if err != nil {
		log.Printf("Failed to watch services: %v", err)
	} else {
		go func() {
			for services := range watchCh {
				fmt.Printf("\nService list updated: now have %d instances\n", len(services))
			}
		}()
	}

	// 7. 健康检查示例
	checker := service_discovery.NewHTTPHealthChecker("/health", 3*time.Second)
	healthManager := service_discovery.NewHealthCheckManager(discovery, checker, 15*time.Second)

	// 注册状态变化回调
	healthManager.RegisterCallback(func(serviceID string, oldStatus, newStatus service_discovery.ServiceStatus) {
		fmt.Printf("Service %s status changed: %s -> %s\n",
			serviceID, oldStatus, newStatus)
	})

	// 添加服务到健康检查
	for _, svc := range services {
		healthManager.AddService(svc)
	}

	// 启动健康检查
	err = healthManager.Start()
	if err != nil {
		log.Printf("Failed to start health check manager: %v", err)
	}
	defer healthManager.Stop()

	// 8. 保持服务活跃（续约）
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 续约服务1
				if err := discovery.KeepAlive(ctx, service1.ID); err != nil {
					log.Printf("Failed to keep alive service %s: %v", service1.ID, err)
				}
				// 续约服务2
				if err := discovery.KeepAlive(ctx, service2.ID); err != nil {
					log.Printf("Failed to keep alive service %s: %v", service2.ID, err)
				}
			}
		}
	}()

	// 9. 列出所有服务
	allServices, err := discovery.ListServices(ctx)
	if err != nil {
		log.Printf("Failed to list all services: %v", err)
	} else {
		fmt.Printf("\nAll registered services (%d total):\n", len(allServices))
		for _, svc := range allServices {
			fmt.Printf("  - %s (%s) at %s\n", svc.Name, svc.ID, svc.Address)
		}
	}

	// 10. 等待一段时间，观察服务发现和健康检查
	fmt.Println("\nWaiting for 30 seconds to observe service discovery...")
	time.Sleep(30 * time.Second)

	// 11. 注销一个服务
	err = discovery.Deregister(ctx, service1.ID)
	if err != nil {
		log.Printf("Failed to deregister service: %v", err)
	} else {
		fmt.Printf("\nDeregistered service: %s\n", service1.ID)
	}

	// 等待观察变化
	time.Sleep(10 * time.Second)

	fmt.Println("\nExample completed.")
}
