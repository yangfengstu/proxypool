package main

import (
	"fmt"
	"log"
	"time"

	"github.com/yourusername/proxypool"
)

func main() {
	fmt.Println("🔧 Dynamic Configuration Example")
	fmt.Println("=================================\n")

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

	fmt.Println("✅ Pool created with initial config:\n")
	printConfig(pool.GetCurrentConfig())

	// 等待一会儿
	time.Sleep(2 * time.Second)

	// 场景1：扩大池子大小
	fmt.Println("\n📈 Scenario 1: Increase pool size")
	newSize := 20
	pool.UpdateConfig(proxypool.UpdateConfig{
		TargetSize: &newSize,
	})
	fmt.Printf("   Updated TargetSize to %d\n", newSize)

	// 场景2：关闭自动刷新
	fmt.Println("\n⏸️  Scenario 2: Disable auto-refresh")
	autoRefresh := false
	pool.UpdateConfig(proxypool.UpdateConfig{
		AutoRefresh: &autoRefresh,
	})
	fmt.Println("   Auto-refresh disabled")

	// 场景3：调整健康阈值
	fmt.Println("\n💊 Scenario 3: Adjust health thresholds")
	minScore := 0.5
	maxFails := 3
	pool.UpdateConfig(proxypool.UpdateConfig{
		MinHealthScore:      &minScore,
		MaxConsecutiveFails: &maxFails,
	})
	fmt.Printf("   MinHealthScore: %.2f\n", minScore)
	fmt.Printf("   MaxConsecutiveFails: %d\n", maxFails)

	// 场景4：调整预检配置
	fmt.Println("\n🧪 Scenario 4: Adjust pre-check config")
	preCheckEnabled := true
	maxLatency := 2 * time.Second
	concurrency := 15
	pool.UpdateConfig(proxypool.UpdateConfig{
		PreCheckEnabled:     &preCheckEnabled,
		PreCheckMaxLatency:  &maxLatency,
		PreCheckConcurrency: &concurrency,
	})
	fmt.Printf("   PreCheck enabled: %v\n", preCheckEnabled)
	fmt.Printf("   MaxLatency: %v\n", maxLatency)
	fmt.Printf("   Concurrency: %d\n", concurrency)

	// 查看最终配置
	fmt.Println("\n📋 Final configuration:")
	printConfig(pool.GetCurrentConfig())

	// 查看池统计
	fmt.Println("\n📊 Pool statistics:")
	stats := pool.Stats()
	fmt.Printf("   Total: %d\n", stats.Total)
	fmt.Printf("   Available: %d\n", stats.Available)
	fmt.Printf("   Utilization: %.2f%%\n", stats.Utilization*100)

	fmt.Println("\n✨ All configurations updated successfully!")
	fmt.Println("\n💡 Key benefits of dynamic configuration:")
	fmt.Println("   • No need to restart the pool")
	fmt.Println("   • Changes take effect immediately")
	fmt.Println("   • Perfect for production hot-reload")
}

func printConfig(cfg proxypool.CurrentConfig) {
	fmt.Printf("   TargetSize: %d\n", cfg.TargetSize)
	fmt.Printf("   LowWatermark: %.2f (%.0f proxies)\n", cfg.LowWatermark, float64(cfg.TargetSize)*cfg.LowWatermark)
	fmt.Printf("   HighWatermark: %.2f (%.0f proxies)\n", cfg.HighWatermark, float64(cfg.TargetSize)*cfg.HighWatermark)
	fmt.Printf("   AutoRefresh: %v\n", cfg.AutoRefresh)
	fmt.Printf("   AutoPrune: %v\n", cfg.AutoPrune)
	fmt.Printf("   MonitorInterval: %v\n", cfg.MonitorInterval)
	fmt.Printf("   PruneInterval: %v\n", cfg.PruneInterval)
	fmt.Printf("   MinHealthScore: %.2f\n", cfg.MinHealthScore)
	fmt.Printf("   MaxConsecutiveFails: %d\n", cfg.MaxConsecutiveFails)
	fmt.Printf("   MaxFailRate: %.2f\n", cfg.MaxFailRate)
	fmt.Printf("   PreCheckEnabled: %v\n", cfg.PreCheckEnabled)
	fmt.Printf("   PreCheckMaxLatency: %v\n", cfg.PreCheckMaxLatency)
	fmt.Printf("   PreCheckConcurrency: %d\n", cfg.PreCheckConcurrency)
}

type simpleLogger struct{}

func (l *simpleLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *simpleLogger) Errorf(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}
