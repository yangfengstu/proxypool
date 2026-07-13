package proxypool

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigApplyDefaultsIncludesPreCheck(t *testing.T) {
	cfg := Config{
		Provider: NewExampleProvider(),
		PreCheck: PreCheckConfig{
			Enabled: true,
		},
	}

	cfg.applyDefaults()

	if cfg.PreCheck.Timeout != 5*time.Second {
		t.Fatalf("PreCheck timeout = %v, want 5s", cfg.PreCheck.Timeout)
	}
	if cfg.PreCheck.MaxLatency != 3*time.Second {
		t.Fatalf("PreCheck max latency = %v, want 3s", cfg.PreCheck.MaxLatency)
	}
	if cfg.PreCheck.Concurrency != 10 {
		t.Fatalf("PreCheck concurrency = %d, want 10", cfg.PreCheck.Concurrency)
	}
	if len(cfg.PreCheck.CheckURLs) == 0 {
		t.Fatal("PreCheck check URLs are empty")
	}
}

func TestPoolDisableCompression(t *testing.T) {
	pool, err := New(Config{
		Provider:           NewExampleProvider(),
		TargetSize:         1,
		DisableCompression: true,
		MonitorInterval:    time.Hour,
		PruneInterval:      time.Hour,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer pool.Close()

	client, _, err := pool.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !client.GetTransport().DisableCompression {
		t.Fatal("transport compression remains enabled")
	}
}

func TestPoolInitializeHonorsStartupConcurrency(t *testing.T) {
	provider := &blockingStartupProvider{
		started: make(chan struct{}, 5),
		release: make(chan struct{}),
	}
	type newResult struct {
		pool *Pool
		err  error
	}
	result := make(chan newResult, 1)

	go func() {
		pool, err := New(Config{
			Provider:           provider,
			TargetSize:         5,
			StartupBatchSize:   1,
			StartupConcurrency: 2,
			MonitorInterval:    time.Hour,
			PruneInterval:      time.Hour,
		})
		result <- newResult{pool: pool, err: err}
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-provider.started:
		case <-time.After(time.Second):
			t.Fatal("startup workers did not begin fetching")
		}
	}
	if got := provider.maxActive.Load(); got != 2 {
		t.Fatalf("maximum concurrent fetches = %d, want 2", got)
	}

	close(provider.release)
	select {
	case res := <-result:
		if res.err != nil {
			t.Fatalf("New() error = %v", res.err)
		}
		defer res.pool.Close()
	case <-time.After(time.Second):
		t.Fatal("pool initialization did not complete")
	}

	if got := provider.maxActive.Load(); got > 2 {
		t.Fatalf("maximum concurrent fetches = %d, want at most 2", got)
	}
}

func TestPoolDynamicPreCheckEnabled(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "precheck failed", http.StatusBadGateway)
	}))
	defer proxyServer.Close()

	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	host, portText, err := net.SplitHostPort(proxyURL.Host)
	if err != nil {
		t.Fatalf("split proxy host: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse proxy port: %v", err)
	}

	pool, err := New(Config{
		Provider:        NewExampleProvider(),
		TargetSize:      1,
		MonitorInterval: time.Hour,
		PruneInterval:   time.Hour,
		PreCheck: PreCheckConfig{
			CheckURLs:       []string{"http://check.invalid"},
			Timeout:         time.Second,
			MaxLatency:      time.Second,
			Concurrency:     1,
			MinSuccessCount: 1,
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer pool.Close()

	enabled := true
	pool.UpdateConfig(UpdateConfig{PreCheckEnabled: &enabled})
	pool.addProxies([]Proxy{{
		Type:      ProxyTypeHTTP,
		Host:      host,
		Port:      port,
		ExpiredAt: time.Now().Add(time.Hour),
	}})

	if got := pool.Stats().Total; got != 1 {
		t.Fatalf("proxy total = %d, want 1 after failed dynamically enabled precheck", got)
	}
}

type blockingStartupProvider struct {
	active    atomic.Int32
	maxActive atomic.Int32
	started   chan struct{}
	release   chan struct{}
}

func (p *blockingStartupProvider) Fetch(ctx context.Context, count int) ([]Proxy, error) {
	active := p.active.Add(1)
	defer p.active.Add(-1)

	for {
		maxActive := p.maxActive.Load()
		if active <= maxActive || p.maxActive.CompareAndSwap(maxActive, active) {
			break
		}
	}
	p.started <- struct{}{}

	select {
	case <-p.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	proxies := make([]Proxy, 0, count)
	for i := 0; i < count; i++ {
		proxies = append(proxies, Proxy{
			Type:      ProxyTypeHTTP,
			Host:      "127.0.0.1",
			Port:      10000 + i,
			ExpiredAt: time.Now().Add(time.Hour),
		})
	}
	return proxies, nil
}

func (p *blockingStartupProvider) Name() string {
	return "blocking-startup"
}
