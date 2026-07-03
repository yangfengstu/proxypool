package main

import (
	"fmt"
	"log"
	"time"

	"github.com/yourusername/proxypool"
)

func main() {
	// 创建代理池
	pool, err := proxypool.New(proxypool.Config{
		Provider:   proxypool.NewExampleProvider(),
		TargetSize: 10,
		Logger:     &simpleLogger{},
	})
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 获取代理客户端
	client, proxy, err := pool.Get()
	if err != nil {
		log.Fatalf("Failed to get proxy: %v", err)
	}

	fmt.Printf("Using proxy: %s\n", proxy.URL())

	// 使用req/v3的方法发起请求
	resp, err := client.R().
		SetHeader("User-Agent", "ProxyPool-Example/1.0").
		Get("https://api.ipify.org?format=json")

	if err != nil {
		pool.ReportFailure(proxy, err)
		log.Fatalf("Request failed: %v", err)
	}

	pool.ReportSuccess(proxy, resp.TotalTime())

	fmt.Printf("Response: %s\n", resp.String())
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Latency: %v\n", resp.TotalTime())

	// 查看池统计
	stats := pool.Stats()
	fmt.Printf("\nPool Stats:\n")
	fmt.Printf("  Total: %d\n", stats.Total)
	fmt.Printf("  Available: %d\n", stats.Available)
	fmt.Printf("  Utilization: %.2f%%\n", stats.Utilization*100)
	fmt.Printf("  Total Requests: %d\n", stats.TotalRequests)
	fmt.Printf("  Success: %d, Failures: %d\n", stats.TotalSuccess, stats.TotalFailures)
}

// simpleLogger 简单的日志实现
type simpleLogger struct{}

func (l *simpleLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *simpleLogger) Errorf(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}
