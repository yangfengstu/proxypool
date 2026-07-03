package proxypool

import (
	"context"
	"time"
)

// startBackgroundTasks 启动后台任务
func (p *Pool) startBackgroundTasks() {
	// 1. 水位线监控任务
	go p.monitorTask()

	// 2. 不健康代理剔除任务
	go p.pruneTask()

	// 3. 健康检查任务（可选）
	if p.config.HealthCheck && p.config.HealthCheckURL != "" {
		go p.healthCheckTask()
	}
}

// monitorTask 水位线监控任务
func (p *Pool) monitorTask() {
	ticker := time.NewTicker(p.config.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.checkAndRefresh()
		case <-p.closeChan:
			return
		}
	}
}

// checkAndRefresh 检查水位线并刷新
func (p *Pool) checkAndRefresh() {
	p.mu.RLock()

	now := time.Now()
	available := 0
	expiringSoon := 0

	for _, pc := range p.proxies {
		if now.Before(pc.ExpireAt) && pc.consecutiveFails < p.config.MaxConsecutiveFails {
			available++

			// 统计即将过期的（在刷新窗口内）
			if pc.ExpireAt.Sub(now) <= p.config.RefreshWindow {
				expiringSoon++
			}
		}
	}

	p.mu.RUnlock()

	// 计算需要补充的数量
	needed := 0
	reason := ""

	// 场景1：可用数量低于低水位（紧急）
	lowWaterCount := int(float64(p.config.TargetSize) * p.config.LowWatermark)
	if available < lowWaterCount {
		needed = p.config.TargetSize - available
		reason = "URGENT: low available"
	}

	// 场景2：即将过期的太多（预防）
	if needed == 0 && expiringSoon > p.config.RefreshBatch {
		needed = min(expiringSoon, p.config.RefreshBatch*2)
		reason = "PROACTIVE: many expiring soon"
	}

	// 场景3：预防性维持高水位
	highWaterCount := int(float64(p.config.TargetSize) * p.config.HighWatermark)
	if needed == 0 && available < highWaterCount {
		needed = highWaterCount - available
		reason = "MAINTAIN: high watermark"
	}

	if needed > 0 {
		p.logf("Refresh triggered: %s, fetching %d proxies (available: %d, expiring soon: %d, target: %d)",
			reason, needed, available, expiringSoon, p.config.TargetSize)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		p.fetchAndAdd(ctx, needed)
	}
}

// pruneTask 不健康代理剔除任务
func (p *Pool) pruneTask() {
	ticker := time.NewTicker(p.config.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.pruneUnhealthyProxies()
		case <-p.closeChan:
			return
		}
	}
}

// pruneUnhealthyProxies 剔除不健康代理
func (p *Pool) pruneUnhealthyProxies() {
	p.mu.Lock()
	defer p.mu.Unlock()

	before := len(p.proxies)
	healthy := make([]*proxyClient, 0, len(p.proxies))

	now := time.Now()

	for _, pc := range p.proxies {
		// 更新健康评分
		pc.updateHealthScore()

		// 剔除条件
		shouldRemove := false

		// 1. 已过期
		if now.After(pc.ExpireAt) {
			shouldRemove = true
		}

		// 2. 健康评分过低
		if !shouldRemove && pc.healthScore < p.config.MinHealthScore {
			shouldRemove = true
			p.logf("Remove proxy %s: low health score %.2f", pc.Proxy.Host, pc.healthScore)
		}

		// 3. 连续失败超限
		if !shouldRemove && pc.consecutiveFails >= p.config.MaxConsecutiveFails {
			shouldRemove = true
			p.logf("Remove proxy %s: consecutive fails %d", pc.Proxy.Host, pc.consecutiveFails)
		}

		// 4. 失败率过高
		if !shouldRemove && pc.useCount >= 10 {
			failRate := float64(pc.failCount) / float64(pc.useCount)
			if failRate > p.config.MaxFailRate {
				shouldRemove = true
				p.logf("Remove proxy %s: high fail rate %.2f", pc.Proxy.Host, failRate)
			}
		}

		if !shouldRemove {
			healthy = append(healthy, pc)
		}
	}

	p.proxies = healthy
	removed := before - len(healthy)

	if removed > 0 {
		p.logf("Pruned %d unhealthy proxies, remaining: %d", removed, len(healthy))
		p.lastPrune.Store(time.Now())

		// 剔除后检查是否需要补充
		go p.checkAndRefresh()
	}
}

// healthCheckTask 健康检查任务
func (p *Pool) healthCheckTask() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performHealthCheck()
		case <-p.closeChan:
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (p *Pool) performHealthCheck() {
	p.mu.RLock()
	proxies := make([]*proxyClient, len(p.proxies))
	copy(proxies, p.proxies)
	p.mu.RUnlock()

	p.logf("Starting health check for %d proxies", len(proxies))

	for _, pc := range proxies {
		go func(pc *proxyClient) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			start := time.Now()
			resp, err := pc.Client.R().SetContext(ctx).Get(p.config.HealthCheckURL)
			latency := time.Since(start)

			if err != nil || resp.StatusCode != 200 {
				p.ReportFailure(&pc.Proxy, err)
			} else {
				p.ReportSuccess(&pc.Proxy, latency)
			}
		}(pc)
	}
}

// fetchAndAdd 拉取并添加代理
func (p *Pool) fetchAndAdd(ctx context.Context, count int) error {
	proxies, err := p.provider.Fetch(ctx, count)
	if err != nil {
		p.errorf("Fetch proxies error: %v", err)
		return err
	}

	p.addProxies(proxies)
	p.logf("Fetched and added %d proxies", len(proxies))
	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
