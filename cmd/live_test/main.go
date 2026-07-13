package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/yangfengstu/proxypool"
	"github.com/yangfengstu/proxypool/providers"
)

var (
	provider   = flag.String("provider", "", "Provider name: ipzan or daili51")
	extractURL = flag.String("url", "", "Extract URL from provider")
	count      = flag.Int("count", 5, "Number of proxies to test")
	testURL    = flag.String("test", "https://api.ipify.org?format=json", "URL to test proxies")
)

func main() {
	flag.Parse()

	if *extractURL == "" {
		log.Fatal("Usage: go run live_test.go -provider=ipzan -url='your_extract_url' [-count=5] [-test=url]")
	}

	fmt.Println("🧪 ProxyPool Live Test")
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Printf("Provider: %s\n", *provider)
	fmt.Printf("Count: %d\n", *count)
	fmt.Printf("Test URL: %s\n\n", *testURL)

	// 创建Provider
	var prov proxypool.Provider
	var err error

	switch *provider {
	case "ipzan":
		prov, err = providers.NewIPZanProvider(providers.IPZanConfig{
			ExtractURL: *extractURL,
		})
	case "daili51":
		prov, err = providers.NewDaili51Provider(providers.Daili51Config{
			ExtractURL: *extractURL,
		})
	default:
		log.Fatal("Invalid provider. Use: ipzan or daili51")
	}

	if err != nil {
		log.Fatalf("❌ Failed to create provider: %v", err)
	}

	fmt.Printf("✅ Provider created: %s\n\n", prov.Name())

	// 测试1: 拉取代理
	fmt.Println("📡 Test 1: Fetching proxies...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	proxies, err := prov.Fetch(ctx, *count)
	if err != nil {
		log.Fatalf("❌ Failed to fetch proxies: %v", err)
	}

	fmt.Printf("✅ Fetched %d proxies\n\n", len(proxies))

	// 显示代理信息
	fmt.Println("📋 Proxy Details:")
	for i, proxy := range proxies {
		fmt.Printf("\n[%d] %s\n", i+1, proxy.URL())
		fmt.Printf("    Type: %s\n", proxy.Type)
		fmt.Printf("    Host: %s:%d\n", proxy.Host, proxy.Port)
		fmt.Printf("    Username: %s\n", proxy.Username)
		fmt.Printf("    Password: %s\n", maskPassword(proxy.Password))
		fmt.Printf("    Expired: %s (%.0f minutes left)\n",
			proxy.ExpiredAt.Format("2006-01-02 15:04:05"),
			time.Until(proxy.ExpiredAt).Minutes())
		if proxy.Region != "" {
			fmt.Printf("    Region: %s\n", proxy.Region)
		}
		if proxy.ISP != "" {
			fmt.Printf("    ISP: %s\n", proxy.ISP)
		}
	}

	// 测试2: 创建代理池
	fmt.Println("\n\n🏊 Test 2: Creating proxy pool...")
	pool, err := proxypool.New(proxypool.Config{
		Provider:   prov,
		TargetSize: *count,
		Logger:     &simpleLogger{},
	})
	if err != nil {
		log.Fatalf("❌ Failed to create pool: %v", err)
	}
	defer pool.Close()

	fmt.Printf("✅ Pool created with %d proxies\n\n", *count)

	// 测试3: 获取代理并发起请求
	fmt.Println("🌐 Test 3: Testing proxy connectivity...")
	successCount := 0
	failCount := 0

	for i := 0; i < min(*count, 3); i++ { // 最多测试3个
		client, proxy, err := pool.Get()
		if err != nil {
			log.Printf("❌ Failed to get proxy: %v", err)
			failCount++
			continue
		}

		fmt.Printf("\n[Test %d] Using proxy: %s\n", i+1, proxy.Host)

		start := time.Now()
		resp, err := client.R().
			SetHeader("User-Agent", "ProxyPool-LiveTest/1.0").
			Get(*testURL)
		latency := time.Since(start)

		if err != nil {
			fmt.Printf("  ❌ Request failed: %v\n", err)
			fmt.Printf("  Latency: %v\n", latency)
			pool.ReportFailure(proxy, err)
			failCount++
			continue
		}

		if resp.StatusCode != 200 {
			fmt.Printf("  ❌ Bad status: %d\n", resp.StatusCode)
			pool.ReportFailure(proxy, fmt.Errorf("status code: %d", resp.StatusCode))
			failCount++
			continue
		}

		fmt.Printf("  ✅ Success! Status: %d\n", resp.StatusCode)
		fmt.Printf("  Latency: %v\n", latency)
		fmt.Printf("  Response: %s\n", truncate(resp.String(), 100))

		pool.ReportSuccess(proxy, latency)
		successCount++
	}

	// 测试4: 查看池统计
	fmt.Println("\n\n📊 Test 4: Pool statistics...")
	stats := pool.Stats()

	fmt.Printf("Total Proxies: %d\n", stats.Total)
	fmt.Printf("Available: %d (%.2f%%)\n", stats.Available, stats.Utilization*100)
	fmt.Printf("Expired: %d\n", stats.Expired)
	fmt.Printf("Unhealthy: %d\n", stats.Unhealthy)
	fmt.Printf("\nRequests: %d\n", stats.TotalRequests)
	fmt.Printf("Success: %d\n", stats.TotalSuccess)
	fmt.Printf("Failures: %d\n", stats.TotalFailures)

	if stats.TotalRequests > 0 {
		successRate := float64(stats.TotalSuccess) / float64(stats.TotalRequests) * 100
		fmt.Printf("Success Rate: %.2f%%\n", successRate)
	}

	// 总结
	fmt.Println("\n\n" + string(make([]byte, 50)) + "=")
	fmt.Println("📝 Test Summary:")
	fmt.Printf("  Provider: %s ✅\n", prov.Name())
	fmt.Printf("  Proxies Fetched: %d ✅\n", len(proxies))
	fmt.Printf("  Connectivity Tests: %d/%d passed\n", successCount, successCount+failCount)

	if successCount > 0 {
		fmt.Println("\n✅ Live test PASSED! Proxies are working.")
	} else {
		fmt.Println("\n❌ Live test FAILED! No successful connections.")
	}
}

func maskPassword(password string) string {
	if len(password) <= 4 {
		return "****"
	}
	return password[:2] + "****" + password[len(password)-2:]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type simpleLogger struct{}

func (l *simpleLogger) Printf(format string, v ...interface{}) {
	log.Printf("[POOL] "+format, v...)
}

func (l *simpleLogger) Errorf(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}
