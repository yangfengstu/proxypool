package proxypool

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/imroc/req/v3"
)

// Pool 代理池
type Pool struct {
	config   Config
	provider Provider
	logger   Logger

	// 代理客户端
	mu         sync.RWMutex
	proxies    []*proxyClient
	nextIndex  int
	closed     bool
	closeChan  chan struct{}
	closeOnce  sync.Once

	// 统计
	totalRequests atomic.Int64
	totalSuccess  atomic.Int64
	totalFailures atomic.Int64
	lastRefresh   atomic.Value // time.Time
	lastPrune     atomic.Value // time.Time
}

// proxyClient 封装的代理客户端
type proxyClient struct {
	Proxy    Proxy
	Client   *req.Client
	ExpireAt time.Time

	// 使用统计
	mu               sync.Mutex
	useCount         int64
	successCount     int64
	failCount        int64
	timeoutCount     int64
	totalLatency     time.Duration
	lastUsed         time.Time
	lastFailed       time.Time
	consecutiveFails int
	healthScore      float64
}

// New 创建代理池
func New(cfg Config) (*Pool, error) {
	// 应用默认值
	cfg.applyDefaults()

	// 验证配置
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	pool := &Pool{
		config:    cfg,
		provider:  cfg.Provider,
		logger:    cfg.Logger,
		proxies:   make([]*proxyClient, 0, cfg.TargetSize),
		closeChan: make(chan struct{}),
	}

	// 初始化：并发快速拉取代理
	if err := pool.initialize(); err != nil {
		return nil, fmt.Errorf("proxypool: initialize failed: %w", err)
	}

	// 启动后台任务
	pool.startBackgroundTasks()

	return pool, nil
}

// initialize 初始化：并发快速拉取代理
func (p *Pool) initialize() error {
	startTime := time.Now()
	p.logf("Initializing proxy pool: target=%d, batchSize=%d, concurrency=%d",
		p.config.TargetSize, p.config.StartupBatchSize, p.config.StartupConcurrency)

	batches := (p.config.TargetSize + p.config.StartupBatchSize - 1) / p.config.StartupBatchSize

	// 并发拉取
	type result struct {
		proxies []Proxy
		err     error
	}

	results := make(chan result, batches)
	ctx := context.Background()

	for i := 0; i < batches; i++ {
		batchSize := p.config.StartupBatchSize
		if i == batches-1 {
			// 最后一批可能少一些
			remaining := p.config.TargetSize - i*p.config.StartupBatchSize
			if remaining < batchSize {
				batchSize = remaining
			}
		}

		go func(batchNo, size int) {
			proxies, err := p.provider.Fetch(ctx, size)
			results <- result{proxies: proxies, err: err}
		}(i, batchSize)
	}

	// 收集结果
	allProxies := make([]Proxy, 0, p.config.TargetSize)
	var errs []error

	for i := 0; i < batches; i++ {
		res := <-results
		if res.err != nil {
			errs = append(errs, res.err)
			p.errorf("Batch fetch error: %v", res.err)
		} else {
			allProxies = append(allProxies, res.proxies...)
		}
	}

	// 添加到池中
	p.addProxies(allProxies)

	elapsed := time.Since(startTime)
	p.logf("Pool initialized: %d proxies in %v (errors: %d)", len(allProxies), elapsed, len(errs))

	if len(allProxies) == 0 {
		return fmt.Errorf("proxypool: no proxies fetched, errors: %v", errs)
	}

	return nil
}

// Get 获取一个可用的代理客户端
// 返回的req.Client已配置好代理，可以直接使用req/v3的所有方法
func (p *Pool) Get() (*req.Client, *Proxy, error) {
	return p.GetWithContext(context.Background())
}

// GetWithContext 带上下文的获取
func (p *Pool) GetWithContext(ctx context.Context) (*req.Client, *Proxy, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, nil, ErrPoolClosed
	}

	// 过滤可用代理（未过期 + 连续失败未超限）
	available := make([]*proxyClient, 0)
	now := time.Now()

	for _, pc := range p.proxies {
		if now.Before(pc.ExpireAt) && pc.consecutiveFails < p.config.MaxConsecutiveFails {
			available = append(available, pc)
		}
	}

	if len(available) == 0 {
		return nil, nil, ErrNoAvailableProxy
	}

	// 按策略选择
	var selected *proxyClient

	switch p.config.SelectStrategy {
	case LeastUsed:
		// 选择使用次数最少的
		sort.Slice(available, func(i, j int) bool {
			return available[i].useCount < available[j].useCount
		})
		selected = available[0]

	case Random:
		// 随机选择
		selected = available[rand.Intn(len(available))]

	case WeightedByHealth:
		// 按健康度加权随机
		selected = p.selectByHealthWeight(available)

	default: // RoundRobin
		// 轮询
		selected = available[p.nextIndex%len(available)]
		p.nextIndex++
	}

	// 更新统计
	selected.mu.Lock()
	selected.useCount++
	selected.lastUsed = time.Now()
	selected.mu.Unlock()

	p.totalRequests.Add(1)

	return selected.Client, &selected.Proxy, nil
}

// selectByHealthWeight 按健康度加权随机选择
func (p *Pool) selectByHealthWeight(available []*proxyClient) *proxyClient {
	// 计算总权重
	totalWeight := 0.0
	for _, pc := range available {
		pc.updateHealthScore()
		totalWeight += pc.healthScore
	}

	if totalWeight == 0 {
		// 如果所有权重都是0，随机选一个
		return available[rand.Intn(len(available))]
	}

	// 加权随机选择
	r := rand.Float64() * totalWeight
	cumulative := 0.0

	for _, pc := range available {
		cumulative += pc.healthScore
		if r <= cumulative {
			return pc
		}
	}

	return available[0]
}

// ReportFailure 报告代理失败（可选，帮助池优化）
func (p *Pool) ReportFailure(proxy *Proxy, err error) {
	if proxy == nil {
		return
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, pc := range p.proxies {
		if pc.Proxy.Host == proxy.Host && pc.Proxy.Port == proxy.Port {
			pc.mu.Lock()
			pc.failCount++
			pc.consecutiveFails++
			pc.lastFailed = time.Now()

			// 判断是否超时错误
			if err != nil && (err.Error() == "timeout" || err.Error() == "context deadline exceeded") {
				pc.timeoutCount++
			}
			pc.mu.Unlock()

			p.totalFailures.Add(1)
			break
		}
	}
}

// ReportSuccess 报告代理成功（可选，帮助池优化）
func (p *Pool) ReportSuccess(proxy *Proxy, latency time.Duration) {
	if proxy == nil {
		return
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, pc := range p.proxies {
		if pc.Proxy.Host == proxy.Host && pc.Proxy.Port == proxy.Port {
			pc.mu.Lock()
			pc.successCount++
			pc.consecutiveFails = 0 // 重置连续失败
			pc.totalLatency += latency
			pc.mu.Unlock()

			p.totalSuccess.Add(1)
			break
		}
	}
}

// Refresh 手动触发刷新
func (p *Pool) Refresh(ctx context.Context) error {
	p.logf("Manual refresh triggered")
	return p.fetchAndAdd(ctx, p.config.RefreshBatch)
}

// Stats 获取统计信息
func (p *Pool) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	stats := Stats{
		Total:         len(p.proxies),
		TotalRequests: p.totalRequests.Load(),
		TotalSuccess:  p.totalSuccess.Load(),
		TotalFailures: p.totalFailures.Load(),
		ProxyStats:    make([]ProxyStats, 0, len(p.proxies)),
	}

	// 统计各类型代理数量
	for _, pc := range p.proxies {
		pc.mu.Lock()

		if now.Before(pc.ExpireAt) {
			if pc.consecutiveFails < p.config.MaxConsecutiveFails && pc.healthScore >= p.config.MinHealthScore {
				stats.Available++
			} else {
				stats.Unhealthy++
			}
		} else {
			stats.Expired++
		}

		// 详细统计
		avgLatency := time.Duration(0)
		if pc.useCount > 0 {
			avgLatency = pc.totalLatency / time.Duration(pc.useCount)
		}

		stats.ProxyStats = append(stats.ProxyStats, ProxyStats{
			Proxy:            pc.Proxy,
			UseCount:         pc.useCount,
			SuccessCount:     pc.successCount,
			FailCount:        pc.failCount,
			TimeoutCount:     pc.timeoutCount,
			HealthScore:      pc.healthScore,
			LastUsed:         pc.lastUsed,
			LastFailed:       pc.lastFailed,
			AvgLatency:       avgLatency,
			ConsecutiveFails: pc.consecutiveFails,
		})

		pc.mu.Unlock()
	}

	// 计算利用率
	if p.config.TargetSize > 0 {
		stats.Utilization = float64(stats.Available) / float64(p.config.TargetSize)
	}

	// 最后操作时间
	if lastRefresh := p.lastRefresh.Load(); lastRefresh != nil {
		stats.LastRefresh = lastRefresh.(time.Time)
	}
	if lastPrune := p.lastPrune.Load(); lastPrune != nil {
		stats.LastPrune = lastPrune.(time.Time)
	}

	return stats
}

// Close 关闭代理池
func (p *Pool) Close() error {
	p.closeOnce.Do(func() {
		p.logf("Closing proxy pool")
		close(p.closeChan)

		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
	})
	return nil
}

// addProxies 添加代理到池中
func (p *Pool) addProxies(proxies []Proxy) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, proxy := range proxies {
		pc := &proxyClient{
			Proxy:       proxy,
			Client:      p.createReqClient(proxy),
			ExpireAt:    proxy.ExpiredAt,
			healthScore: 1.0, // 新代理默认满分
		}
		p.proxies = append(p.proxies, pc)
	}

	p.lastRefresh.Store(time.Now())
}

// createReqClient 创建配置好的req.Client
func (p *Pool) createReqClient(proxy Proxy) *req.Client {
	client := req.C()

	// 设置代理
	client.SetProxyURL(proxy.URL())

	// 连接复用配置
	if p.config.EnableKeepAlive {
		client.EnableKeepAlives()
	} else {
		client.DisableKeepAlives()
	}

	// 连接池配置
	// 注意：req/v3 v3.43.7+ 提供了这些方法
	transport := client.GetTransport()
	transport.MaxIdleConns = p.config.MaxIdleConns
	transport.MaxIdleConnsPerHost = p.config.MaxIdleConnsPerHost
	transport.IdleConnTimeout = p.config.IdleConnTimeout
	transport.TLSHandshakeTimeout = p.config.TLSHandshakeTimeout

	// 超时配置
	client.SetTimeout(p.config.Timeout)

	// 其他配置
	if p.config.DisableCompression {
		client.DisableAutoReadResponse()
	}

	// 重定向
	client.SetRedirectPolicy(req.MaxRedirectPolicy(p.config.MaxRedirects))

	// 不自动重试（让调用层控制）
	client.SetCommonRetryCount(0)

	return client
}

// logf 输出日志
func (p *Pool) logf(format string, v ...interface{}) {
	if p.logger != nil {
		p.logger.Printf(format, v...)
	}
}

// errorf 输出错误日志
func (p *Pool) errorf(format string, v ...interface{}) {
	if p.logger != nil {
		p.logger.Errorf(format, v...)
	}
}

// updateHealthScore 更新健康评分
func (pc *proxyClient) updateHealthScore() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.useCount == 0 {
		pc.healthScore = 1.0
		return
	}

	// 成功率（60%权重）
	successRate := float64(pc.successCount) / float64(pc.useCount)

	// 超时率（20%权重）
	timeoutRate := float64(pc.timeoutCount) / float64(pc.useCount)

	// 平均延迟（20%权重）
	avgLatency := pc.totalLatency / time.Duration(pc.useCount)
	latencyScore := 1.0
	if avgLatency > 5*time.Second {
		latencyScore = 0.3
	} else if avgLatency > 3*time.Second {
		latencyScore = 0.6
	} else if avgLatency > 1*time.Second {
		latencyScore = 0.8
	}

	// 综合评分
	score := successRate*0.6 - timeoutRate*0.2 + latencyScore*0.2

	// 连续失败惩罚
	if pc.consecutiveFails >= 3 {
		score = score * 0.5
	}

	// 确保在 0-1 范围内
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	pc.healthScore = score
}
