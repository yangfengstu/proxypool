package main

import (
	"fmt"
	"log"

	"github.com/yourusername/proxypool"
)

func main() {
	fmt.Println("📊 Proxy Export & Logging Example")
	fmt.Println("==================================\n")

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

	// 使用几个代理
	fmt.Println("🔄 Using some proxies...\n")
	for i := 0; i < 3; i++ {
		client, proxy, _ := pool.Get()
		fmt.Printf("[Request %d] Using: %s\n", i+1, proxy.SafeURL())

		// 获取代理信息（用于日志）
		info := pool.GetProxyInfo(proxy)
		fmt.Printf("            Info: %s\n", info.String())

		// 模拟使用
		_ = client
	}

	// 场景1：获取所有代理详情
	fmt.Println("\n\n📋 Scenario 1: Get all proxy details")
	fmt.Println("─────────────────────────────────────")
	details := pool.GetAllProxyDetails()
	for i, detail := range details {
		if i >= 3 {
			break // 只显示前3个
		}
		fmt.Printf("\n[Proxy %d]\n", i+1)
		fmt.Printf("  Address: %s:%d (%s)\n", detail.ProxyHost, detail.ProxyPort, detail.ProxyType)
		if detail.RealExitIP != "" {
			fmt.Printf("  Exit IP: %s (from: %s)\n", detail.RealExitIP, detail.ExitIPFrom)
		}
		fmt.Printf("  Region: %s | ISP: %s\n", detail.Region, detail.ISP)
		fmt.Printf("  Expires: %s (%s)\n", detail.ExpiredAt.Format("15:04:05"), detail.TimeToExpire)
		fmt.Printf("  Usage: %d total, %d success (%.1f%%)\n",
			detail.UseCount, detail.SuccessCount, detail.SuccessRate)
		fmt.Printf("  Health: %.2f | Consecutive fails: %d\n",
			detail.HealthScore, detail.ConsecutiveFails)
		if detail.AvgLatency != "" && detail.AvgLatency != "0s" {
			fmt.Printf("  Avg Latency: %s\n", detail.AvgLatency)
		}
	}

	// 场景2：导出为JSON
	fmt.Println("\n\n📤 Scenario 2: Export as JSON")
	fmt.Println("─────────────────────────────────────")
	jsonData, err := pool.ExportProxyDetailsJSON()
	if err != nil {
		log.Printf("Export error: %v", err)
	} else {
		// 只显示前300个字符
		if len(jsonData) > 300 {
			fmt.Printf("%s\n... (truncated)\n", jsonData[:300])
		} else {
			fmt.Println(jsonData)
		}
	}

	// 场景3：根据地址查找代理
	fmt.Println("\n\n🔍 Scenario 3: Find proxy by address")
	fmt.Println("─────────────────────────────────────")
	if len(details) > 0 {
		first := details[0]
		found := pool.GetProxyByHost(first.ProxyHost, first.ProxyPort)
		if found != nil {
			fmt.Printf("Found proxy: %s:%d\n", found.ProxyHost, found.ProxyPort)
			if found.RealExitIP != "" {
				fmt.Printf("Exit IP: %s\n", found.RealExitIP)
			}
			fmt.Printf("Health: %.2f | Use count: %d\n", found.HealthScore, found.UseCount)
		}
	}

	// 场景4：在日志中打印代理信息
	fmt.Println("\n\n📝 Scenario 4: Logging proxy info")
	fmt.Println("─────────────────────────────────────")
	_, proxy, _ := pool.Get()
	info := pool.GetProxyInfo(proxy)

	// 模拟日志输出
	log.Printf("Request started %s", info)
	log.Printf("Request completed successfully %s", info)

	fmt.Println("\n✨ All scenarios completed!")
}

type simpleLogger struct{}

func (l *simpleLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *simpleLogger) Errorf(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}
