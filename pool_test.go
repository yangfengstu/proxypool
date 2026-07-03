package proxypool_test

import (
	"testing"
	"time"

	"github.com/yourusername/proxypool"
)

func TestPool_Basic(t *testing.T) {
	// 创建代理池
	pool, err := proxypool.New(proxypool.Config{
		Provider:   proxypool.NewExampleProvider(),
		TargetSize: 10,
		Logger:     &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 获取代理
	client, proxy, err := pool.Get()
	if err != nil {
		t.Fatalf("Failed to get proxy: %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	if proxy == nil {
		t.Fatal("Proxy is nil")
	}

	t.Logf("Got proxy: %s", proxy.URL())

	// 检查统计
	stats := pool.Stats()
	if stats.Total < 10 {
		t.Errorf("Expected at least 10 proxies, got %d", stats.Total)
	}

	t.Logf("Pool stats: Total=%d, Available=%d, Utilization=%.2f%%",
		stats.Total, stats.Available, stats.Utilization*100)
}

func TestPool_ReportFailureAndSuccess(t *testing.T) {
	pool, err := proxypool.New(proxypool.Config{
		Provider:   proxypool.NewExampleProvider(),
		TargetSize: 5,
		Logger:     &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 获取代理
	client, proxy, err := pool.Get()
	if err != nil {
		t.Fatalf("Failed to get proxy: %v", err)
	}

	// 报告失败
	pool.ReportFailure(proxy, nil)

	// 报告成功
	pool.ReportSuccess(proxy, 100*time.Millisecond)

	// 检查统计
	stats := pool.Stats()
	if len(stats.ProxyStats) == 0 {
		t.Fatal("No proxy stats")
	}

	found := false
	for _, ps := range stats.ProxyStats {
		if ps.Proxy.Host == proxy.Host {
			found = true
			if ps.SuccessCount != 1 {
				t.Errorf("Expected success count 1, got %d", ps.SuccessCount)
			}
			if ps.FailCount != 1 {
				t.Errorf("Expected fail count 1, got %d", ps.FailCount)
			}
			t.Logf("Proxy stats: UseCount=%d, Success=%d, Fail=%d, HealthScore=%.2f",
				ps.UseCount, ps.SuccessCount, ps.FailCount, ps.HealthScore)
			break
		}
	}

	if !found {
		t.Error("Proxy stats not found")
	}

	_ = client // 避免未使用警告
}

func TestPool_SelectStrategies(t *testing.T) {
	strategies := map[string]proxypool.SelectStrategy{
		"RoundRobin": proxypool.RoundRobin,
		"LeastUsed":  proxypool.LeastUsed,
		"Random":     proxypool.Random,
	}

	for name, strategy := range strategies {
		t.Run(name, func(t *testing.T) {
			pool, err := proxypool.New(proxypool.Config{
				Provider:       proxypool.NewExampleProvider(),
				TargetSize:     5,
				SelectStrategy: strategy,
				Logger:         &testLogger{t: t},
			})
			if err != nil {
				t.Fatalf("Failed to create pool: %v", err)
			}
			defer pool.Close()

			// 获取多个代理，测试选择策略
			for i := 0; i < 10; i++ {
				_, proxy, err := pool.Get()
				if err != nil {
					t.Fatalf("Failed to get proxy: %v", err)
				}
				t.Logf("Round %d: %s", i+1, proxy.URL())
			}
		})
	}
}

// testLogger 测试用的日志实现
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}

func (l *testLogger) Errorf(format string, v ...interface{}) {
	l.t.Errorf(format, v...)
}
