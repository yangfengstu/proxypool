package main

import (
	"fmt"
	"log"
	"time"

	"github.com/yourusername/proxypool"
	"github.com/yourusername/proxypool/providers"
)

func main() {
	// 创建IP赞提供商
	provider, err := providers.NewIPZanProvider(providers.IPZanConfig{
		ExtractURL: "https://service.ipzan.com/core-extract?num=1&no=xxx&minute=3&format=json&protocol=3&pool=quality&mode=auth&secret=xxx",
	})
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	fmt.Println("🧪 Testing Proxy Pre-Check Feature")
	fmt.Println("====================================\n")

	// 创建代理池（启用预检）
	pool, err := proxypool.New(proxypool.Config{
		Provider:   provider,
		TargetSize: 10,

		// ✨ 启用预检
		PreCheck: proxypool.PreCheckConfig{
			Enabled:         true,                // 启用预检
			Timeout:         5 * time.Second,     // 预检超时5秒
			MaxLatency:      3 * time.Second,     // 最大延迟3秒
			Concurrency:     5,                   // 5个并发worker
			RequireRealIP:   true,                // 必须获取到真实IP
			MinSuccessCount: 1,                   // 至少成功1次
			// CheckURLs 使用默认列表
		},

		Logger: &simpleLogger{},
	})
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	fmt.Println("\n✅ Pool initialized with pre-check enabled!")

	// 查看池统计
	stats := pool.Stats()
	fmt.Printf("\n📊 Pool Statistics:\n")
	fmt.Printf("   Total: %d (after pre-check)\n", stats.Total)
	fmt.Printf("   Available: %d\n", stats.Available)

	// 查看预检结果
	if len(stats.ProxyStats) > 0 {
		fmt.Printf("\n🔍 Pre-Check Results:\n")
		for i, ps := range stats.ProxyStats {
			if i >= 3 {
				break // 只显示前3个
			}
			fmt.Printf("\n[%d] %s\n", i+1, ps.Proxy.Host)
			if latency := ps.Proxy.Metadata["precheck_latency"]; latency != "" {
				fmt.Printf("    Pre-check Latency: %s\n", latency)
			}
			if realIP := ps.Proxy.Metadata["precheck_real_ip"]; realIP != "" {
				fmt.Printf("    Real IP: %s\n", realIP)
			}
			if checkedAt := ps.Proxy.Metadata["precheck_at"]; checkedAt != "" {
				fmt.Printf("    Checked At: %s\n", checkedAt)
			}
		}
	}

	// 使用代理
	fmt.Println("\n🌐 Using a proxy from the pool...")
	client, proxy, err := pool.Get()
	if err != nil {
		log.Fatalf("Failed to get proxy: %v", err)
	}

	fmt.Printf("   Using: %s (Pre-checked ✓)\n", proxy.Host)

	resp, err := client.R().Get("https://api.ipify.org?format=json")
	if err != nil {
		log.Printf("Request failed: %v", err)
	} else {
		fmt.Printf("   Response: %s\n", resp.String())
		fmt.Printf("   Status: %d\n", resp.StatusCode)
	}

	fmt.Println("\n✨ All proxies in the pool have been pre-checked and verified!")
}

type simpleLogger struct{}

func (l *simpleLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *simpleLogger) Errorf(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}
