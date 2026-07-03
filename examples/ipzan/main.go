package main

import (
	"fmt"
	"log"

	"github.com/yourusername/proxypool"
	"github.com/yourusername/proxypool/providers"
)

func main() {
	// 创建IP赞提供商
	ipzanProvider, err := providers.NewIPZanProvider(providers.IPZanConfig{
		No:       "20241017192301118136", // 你的订单号
		Secret:   "u0co3e3ea1e815o",      // 你的密钥
		Minute:   3,                      // 代理有效期3分钟
		Protocol: 3,                      // SOCKS5
	})
	if err != nil {
		log.Fatalf("Failed to create IPZan provider: %v", err)
	}

	// 创建代理池
	pool, err := proxypool.New(proxypool.Config{
		Provider:   ipzanProvider,
		TargetSize: 100, // 维持100个代理
		Logger:     &simpleLogger{},
	})
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	fmt.Println("✅ Proxy pool initialized successfully!")

	// 获取代理并使用
	client, proxy, err := pool.Get()
	if err != nil {
		log.Fatalf("Failed to get proxy: %v", err)
	}

	fmt.Printf("\n🔌 Using proxy: %s\n", proxy.URL())
	fmt.Printf("   Type: %s\n", proxy.Type)
	fmt.Printf("   ISP: %s\n", proxy.ISP)
	fmt.Printf("   Expired at: %s\n", proxy.ExpiredAt.Format("2006-01-02 15:04:05"))

	// 使用代理发起请求
	resp, err := client.R().Get("https://api.ipify.org?format=json")
	if err != nil {
		pool.ReportFailure(proxy, err)
		log.Fatalf("Request failed: %v", err)
	}

	pool.ReportSuccess(proxy, resp.TotalTime())

	fmt.Printf("\n📡 Request successful!\n")
	fmt.Printf("   Response: %s\n", resp.String())
	fmt.Printf("   Status: %d\n", resp.StatusCode)
	fmt.Printf("   Latency: %v\n", resp.TotalTime())

	// 查看池统计
	stats := pool.Stats()
	fmt.Printf("\n📊 Pool Statistics:\n")
	fmt.Printf("   Total: %d\n", stats.Total)
	fmt.Printf("   Available: %d (%.2f%%)\n", stats.Available, stats.Utilization*100)
	fmt.Printf("   Total Requests: %d\n", stats.TotalRequests)
	fmt.Printf("   Success: %d, Failures: %d\n", stats.TotalSuccess, stats.TotalFailures)
}

type simpleLogger struct{}

func (l *simpleLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *simpleLogger) Errorf(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}
