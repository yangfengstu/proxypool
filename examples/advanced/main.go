package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yangfengstu/proxypool"
)

func main() {
	// 创建自定义提供商
	provider := NewCustomProvider()

	// 创建代理池（完整配置）
	pool, err := proxypool.New(proxypool.Config{
		// 必需
		Provider: provider,

		// 池大小管理
		TargetSize:    100,
		LowWatermark:  0.7,
		HighWatermark: 0.9,

		// 启动配置
		StartupBatchSize:   20,
		StartupConcurrency: 5,

		// 运行时刷新
		RefreshWindow: 15 * time.Minute,
		RefreshBatch:  20,

		// 选择策略
		SelectStrategy: proxypool.WeightedByHealth,

		// HTTP客户端配置
		EnableKeepAlive:     true,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		Timeout:             5 * time.Second,

		// 健康检查
		HealthCheck:         true,
		HealthCheckURL:      "https://api.ipify.org",
		HealthCheckInterval: 5 * time.Minute,
		MinHealthScore:      0.3,
		MaxConsecutiveFails: 5,
		MaxFailRate:         0.8,

		// 日志
		Logger: &simpleLogger{},
	})
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 模拟多次请求
	for i := 0; i < 10; i++ {
		makeRequest(pool, i+1)
		time.Sleep(1 * time.Second)
	}

	// 输出最终统计
	printStats(pool)
}

func makeRequest(pool *proxypool.Pool, round int) {
	client, proxy, err := pool.Get()
	if err != nil {
		log.Printf("Round %d: Failed to get proxy: %v", round, err)
		return
	}

	fmt.Printf("Round %d: Using proxy %s\n", round, proxy.URL())

	start := time.Now()
	resp, err := client.R().Get("https://api.ipify.org?format=json")
	latency := time.Since(start)

	if err != nil {
		pool.ReportFailure(proxy, err)
		log.Printf("Round %d: Request failed: %v", round, err)
		return
	}

	pool.ReportSuccess(proxy, latency)
	fmt.Printf("Round %d: Success! Latency: %v, Response: %s\n", round, latency, resp.String())
}

func printStats(pool *proxypool.Pool) {
	stats := pool.Stats()

	fmt.Println("\n========== Pool Statistics ==========")
	fmt.Printf("Total Proxies: %d\n", stats.Total)
	fmt.Printf("Available: %d (%.2f%%)\n", stats.Available, stats.Utilization*100)
	fmt.Printf("Expired: %d\n", stats.Expired)
	fmt.Printf("Unhealthy: %d\n", stats.Unhealthy)
	fmt.Println()
	fmt.Printf("Total Requests: %d\n", stats.TotalRequests)
	fmt.Printf("Success: %d\n", stats.TotalSuccess)
	fmt.Printf("Failures: %d\n", stats.TotalFailures)

	if stats.TotalRequests > 0 {
		successRate := float64(stats.TotalSuccess) / float64(stats.TotalRequests) * 100
		fmt.Printf("Success Rate: %.2f%%\n", successRate)
	}

	fmt.Println("\n========== Top 5 Proxies ==========")
	for i, ps := range stats.ProxyStats {
		if i >= 5 {
			break
		}
		fmt.Printf("%d. %s:%d\n", i+1, ps.Proxy.Host, ps.Proxy.Port)
		fmt.Printf("   UseCount: %d, Success: %d, Fail: %d\n", ps.UseCount, ps.SuccessCount, ps.FailCount)
		fmt.Printf("   HealthScore: %.2f, AvgLatency: %v\n", ps.HealthScore, ps.AvgLatency)
	}
}

// CustomProvider 自定义提供商
type CustomProvider struct{}

func NewCustomProvider() *CustomProvider {
	return &CustomProvider{}
}

func (p *CustomProvider) Fetch(ctx context.Context, count int) ([]proxypool.Proxy, error) {
	// 这里应该调用真实的代理商API
	// 示例：返回模拟数据
	proxies := make([]proxypool.Proxy, 0, count)

	for i := 0; i < count; i++ {
		proxies = append(proxies, proxypool.Proxy{
			Type:      proxypool.ProxyTypeSOCKS5,
			Host:      fmt.Sprintf("proxy%d.example.com", i+1),
			Port:      1080,
			Username:  "user",
			Password:  "pass",
			ExpiredAt: time.Now().Add(60 * time.Minute),
			Region:    "US",
		})
	}

	return proxies, nil
}

func (p *CustomProvider) Name() string {
	return "custom-provider"
}

type simpleLogger struct{}

func (l *simpleLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *simpleLogger) Errorf(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}
