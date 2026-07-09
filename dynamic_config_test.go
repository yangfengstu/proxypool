package proxypool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestDynamicConfig_Update(t *testing.T) {
	pool, err := New(Config{
		Provider:   NewExampleProvider(),
		TargetSize: 10,
		Logger:     &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 获取初始配置
	cfg := pool.GetCurrentConfig()
	if cfg.TargetSize != 10 {
		t.Errorf("Expected TargetSize 10, got %d", cfg.TargetSize)
	}
	if !cfg.AutoRefresh {
		t.Error("Expected AutoRefresh to be true")
	}

	// 更新配置
	newSize := 20
	autoRefresh := false
	pool.UpdateConfig(UpdateConfig{
		TargetSize:  &newSize,
		AutoRefresh: &autoRefresh,
	})

	// 验证更新
	cfg = pool.GetCurrentConfig()
	if cfg.TargetSize != 20 {
		t.Errorf("Expected TargetSize 20, got %d", cfg.TargetSize)
	}
	if cfg.AutoRefresh {
		t.Error("Expected AutoRefresh to be false")
	}
}

func TestDynamicConfig_HealthThresholds(t *testing.T) {
	pool, err := New(Config{
		Provider:            NewExampleProvider(),
		TargetSize:          5,
		MinHealthScore:      0.3,
		MaxConsecutiveFails: 5,
		MaxFailRate:         0.8,
		Logger:              &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 更新健康阈值
	minScore := 0.5
	maxFails := 3
	maxRate := 0.6

	pool.UpdateConfig(UpdateConfig{
		MinHealthScore:      &minScore,
		MaxConsecutiveFails: &maxFails,
		MaxFailRate:         &maxRate,
	})

	// 验证
	cfg := pool.GetCurrentConfig()
	if cfg.MinHealthScore != 0.5 {
		t.Errorf("Expected MinHealthScore 0.5, got %.2f", cfg.MinHealthScore)
	}
	if cfg.MaxConsecutiveFails != 3 {
		t.Errorf("Expected MaxConsecutiveFails 3, got %d", cfg.MaxConsecutiveFails)
	}
	if cfg.MaxFailRate != 0.6 {
		t.Errorf("Expected MaxFailRate 0.6, got %.2f", cfg.MaxFailRate)
	}
}

func TestDynamicConfig_PreCheck(t *testing.T) {
	pool, err := New(Config{
		Provider:   NewExampleProvider(),
		TargetSize: 5,
		PreCheck: PreCheckConfig{
			Enabled:    true,
			MaxLatency: 3 * time.Second,
		},
		Logger: &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 更新预检配置
	enabled := false
	maxLatency := 5 * time.Second
	concurrency := 20

	pool.UpdateConfig(UpdateConfig{
		PreCheckEnabled:     &enabled,
		PreCheckMaxLatency:  &maxLatency,
		PreCheckConcurrency: &concurrency,
	})

	// 验证
	cfg := pool.GetCurrentConfig()
	if cfg.PreCheckEnabled {
		t.Error("Expected PreCheckEnabled to be false")
	}
	if cfg.PreCheckMaxLatency != 5*time.Second {
		t.Errorf("Expected PreCheckMaxLatency 5s, got %v", cfg.PreCheckMaxLatency)
	}
	if cfg.PreCheckConcurrency != 20 {
		t.Errorf("Expected PreCheckConcurrency 20, got %d", cfg.PreCheckConcurrency)
	}
}

func TestDynamicConfig_DisableAutoRefresh(t *testing.T) {
	pool, err := New(Config{
		Provider:   NewExampleProvider(),
		TargetSize: 5,
		Logger:     &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 关闭自动刷新
	autoRefresh := false
	pool.UpdateConfig(UpdateConfig{
		AutoRefresh: &autoRefresh,
	})

	// 等待一段时间，确保不会自动刷新
	time.Sleep(100 * time.Millisecond)

	// 手动触发刷新应该仍然可用
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = pool.Refresh(ctx)
	if err != nil {
		t.Errorf("Manual refresh failed: %v", err)
	}
}

func TestPruneSkipsRefreshWhenAutoRefreshDisabled(t *testing.T) {
	provider := &countingProvider{fetches: make(chan int, 8)}
	pool, err := New(Config{
		Provider:        provider,
		TargetSize:      2,
		MonitorInterval: time.Hour,
		PruneInterval:   time.Hour,
		Logger:          &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()
	drainFetches(provider.fetches)

	autoRefresh := false
	pool.UpdateConfig(UpdateConfig{AutoRefresh: &autoRefresh})
	expireAllProxies(pool)
	before := provider.calls.Load()

	pool.pruneUnhealthyProxies()

	select {
	case <-provider.fetches:
		t.Fatal("prune should not trigger refresh when AutoRefresh is disabled")
	case <-time.After(100 * time.Millisecond):
	}
	if got := provider.calls.Load(); got != before {
		t.Fatalf("fetch calls = %d, want %d", got, before)
	}
}

func TestPruneRefreshesWhenAutoRefreshEnabled(t *testing.T) {
	provider := &countingProvider{fetches: make(chan int, 8)}
	pool, err := New(Config{
		Provider:        provider,
		TargetSize:      2,
		MonitorInterval: time.Hour,
		PruneInterval:   time.Hour,
		Logger:          &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()
	drainFetches(provider.fetches)

	expireAllProxies(pool)
	pool.pruneUnhealthyProxies()

	select {
	case <-provider.fetches:
	case <-time.After(time.Second):
		t.Fatal("prune should trigger refresh when AutoRefresh is enabled")
	}
}

type countingProvider struct {
	calls   atomic.Int32
	fetches chan int
}

func (p *countingProvider) Fetch(ctx context.Context, count int) ([]Proxy, error) {
	p.calls.Add(1)
	select {
	case p.fetches <- count:
	default:
	}
	proxies := make([]Proxy, 0, count)
	for i := 0; i < count; i++ {
		proxies = append(proxies, Proxy{
			Type:      ProxyTypeSOCKS5,
			Host:      "127.0.0.1",
			Port:      10000 + i,
			ExpiredAt: time.Now().Add(time.Hour),
		})
	}
	return proxies, nil
}

func (p *countingProvider) Name() string {
	return "counting"
}

func expireAllProxies(pool *Pool) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	expiredAt := time.Now().Add(-time.Second)
	for _, proxy := range pool.proxies {
		proxy.ExpireAt = expiredAt
		proxy.Proxy.ExpiredAt = expiredAt
	}
}

func drainFetches(fetches <-chan int) {
	for {
		select {
		case <-fetches:
		default:
			return
		}
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
